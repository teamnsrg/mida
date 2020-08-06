package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/teamnsrg/mida/amqp"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/fetch"
	"github.com/teamnsrg/mida/log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// stage1 is the top level function of stage 1 of the MIDA pipeline and is responsible
// for getting the raw tasks (from any source) and placing them into the raw task channel.
func stage1(rtc chan<- *b.RawTask, cmd *cobra.Command, args []string) {

	// Rate limit the beginning of tasks. This prevents all parallel browsers from opening at the
	// same time, straining system resources.
	rateLimiter := time.Tick(time.Duration(viper.GetInt("rate_limit")) * time.Millisecond)

	switch cmd.Name() {
	case "file":
		rawTasks, err := fetch.FromFile(args[0], viper.GetBool("shuffle"))
		if err != nil {
			log.Log.Error(err)
			close(rtc)
			return
		}
		for rt := range rawTasks {
			rtCopy := rt
			rtc <- rtCopy
			<-rateLimiter
		}

	case "go":
		// Generate our task set from command line options, decompress it,
		// and load our tasks into the pipeline.
		cts, err := BuildCompressedTaskSet(cmd, args)
		if err != nil {
			log.Log.Error(err)
			close(rtc)
			return
		}

		rawTasks := b.ExpandCompressedTaskSet(*cts)
		for _, rt := range rawTasks {
			rtCopy := rt
			rtc <- &rtCopy
			<-rateLimiter
		}

	case "client":
		var params = amqp.ConnParams{
			User: viper.GetString("amqp_user"),
			Pass: viper.GetString("amqp_pass"),
			Uri:  viper.GetString("amqp_uri"),
		}

		// Register a signal handler so we can gracefully exit on SIGTERM
		sigChan := make(chan os.Signal, 5)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		taskAMQPConn, taskDeliveryChan, err := amqp.NewAMQPTasksConsumer(params, viper.GetString("amqp_task_queue"))
		if err != nil {
			log.Log.Fatal(err)
		}
		defer taskAMQPConn.Shutdown()

		broadcastAMQPConn, broadcastAMQPDeliveryChan, err := amqp.NewAMQPBroadcastConsumer(params, amqp.DefaultBroadcastExchange)
		if err != nil {
			log.Log.Fatal(err)
		}
		defer broadcastAMQPConn.Shutdown()

		log.Log.Infof("Successfully connected to AMQP Queue: \"%s\"", viper.GetString("amqp_task_queue"))

		// Remain as a client to the AMQP server until a broadcast is received which
		// causes us to exit
		breakFlag := false
		for {
			select {
			case broadcastMsg := <-broadcastAMQPDeliveryChan:
				log.Log.Warnf("BROADCAST RECEIVED: [ %s ]", string(broadcastMsg.Body))
				if string(broadcastMsg.Body) == "quit" {
					breakFlag = true
				}
			case <-sigChan:
				log.Log.Warn("Received SIGTERM, will not start any more tasks")
				log.Log.Warn("Press Ctrl+C again to kill MIDA immediately")
				signal.Reset() // If ctrl+C is pressed again, we just die
				breakFlag = true
			default:
			}
			select {
			case broadcastMsg := <-broadcastAMQPDeliveryChan:
				log.Log.Warnf("BROADCAST RECEIVED: [ %s ]", string(broadcastMsg.Body))
				if string(broadcastMsg.Body) == "quit" {
					breakFlag = true
				}
			case <-sigChan:
				log.Log.Warn("Received SIGTERM, will not start any more tasks")
				log.Log.Warn("Press Ctrl+C again to kill MIDA immediately")
				signal.Reset() // If ctrl+C is pressed again, we just die
				breakFlag = true
			case amqpMsg := <-taskDeliveryChan:
				rawTask, err := amqp.DecodeAMQPMessageToRawTask(amqpMsg)
				if err != nil {
					log.Log.Error(err)
				}
				rtc <- &rawTask
				<-rateLimiter
			}
			if breakFlag {
				break
			}
		}
	}

	// Close the task channel after we have dumped all tasks into it
	close(rtc)
}
