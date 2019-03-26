package main

import (
	"encoding/json"
	"errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/queue"
	t "github.com/teamnsrg/mida/types"
	"io/ioutil"
)

// Reads in a single task or task list from a byte array
func ReadTasks(data []byte) ([]t.MIDATask, error) {
	tasks := make(t.MIDATaskSet, 0)
	err := json.Unmarshal(data, &tasks)
	if err == nil {
		log.Log.Debug("Parsed MIDATaskSet from file")
		return tasks, nil
	}

	singleTask := t.MIDATask{}
	err = json.Unmarshal(data, &singleTask)
	if err == nil {
		log.Log.Debug("Parsed single MIDATask from file")
		return append(tasks, singleTask), nil
	}

	compressedTaskSet := t.CompressedMIDATaskSet{}
	err = json.Unmarshal(data, &compressedTaskSet)
	if err != nil {
		return tasks, errors.New("failed to unmarshal tasks")
	}

	if compressedTaskSet.URL == nil || len(*compressedTaskSet.URL) == 0 {
		return tasks, errors.New("no URLs given in task set")
	}
	tasks = ExpandCompressedTaskSet(compressedTaskSet)

	log.Log.Debug("Parsed CompressedMIDATaskSet from file")
	return tasks, nil

}

// Wrapper function that reads single tasks, full task sets,
// or compressed task sets from file
func ReadTasksFromFile(fName string) ([]t.MIDATask, error) {
	tasks := make(t.MIDATaskSet, 0)

	data, err := ioutil.ReadFile(fName)
	if err != nil {
		return tasks, errors.New("failed to read task file: " + fName)
	}

	tasks, err = ReadTasks(data)
	if err != nil {
		return tasks, err
	}

	return tasks, nil
}

func ExpandCompressedTaskSet(ts t.CompressedMIDATaskSet) []t.MIDATask {
	var rawTasks []t.MIDATask
	for _, v := range *ts.URL {
		urlString := v
		newTask := t.MIDATask{
			URL:         &urlString,
			Browser:     ts.Browser,
			Completion:  ts.Completion,
			Data:        ts.Data,
			Output:      ts.Output,
			MaxAttempts: ts.MaxAttempts,
		}
		rawTasks = append(rawTasks, newTask)
	}
	return rawTasks
}

// Retrieves raw tasks, either from a queue, file, or pre-built set
func TaskIntake(rtc chan<- t.MIDATask, cmd *cobra.Command, args []string) {
	if cmd.Name() == "client" {
		// TODO: Figure out how to close connection gracefully here
		taskAMQPConn, taskDeliveryChan, err := queue.NewAMQPTasksConsumer()
		if err != nil {
			log.Log.Fatal(err)
		}
		defer taskAMQPConn.Shutdown()

		broadcastAMQPConn, broadcastAMQPDeliveryChan, err := queue.NewAMQPBroadcastConsumer()
		if err != nil {
			log.Log.Fatal(err)
		}
		defer broadcastAMQPConn.Shutdown()

		// Remain as a client to the AMQP server until a broadcast is received which
		// causes us to exit
		breakFlag := false
		for {
			select {
			case broadcastMsg := <-broadcastAMQPDeliveryChan:
				log.Log.Warnf("BROADCAST RECEIVED: [ %s ]", string(broadcastMsg.Body))
				breakFlag = true
			default:
			}
			select {
			case broadcastMsg := <-broadcastAMQPDeliveryChan:
				log.Log.Warnf("BROADCAST RECEIVED: [ %s ]", string(broadcastMsg.Body))
				breakFlag = true
			case amqpMsg := <-taskDeliveryChan:
				rawTask, err := queue.DecodeAMQPMessageToRawTask(amqpMsg)
				if err != nil {
					log.Log.Fatal(err)
				}
				rtc <- rawTask
			}
			if breakFlag {
				break
			}
		}

	} else if cmd.Name() == "file" {
		rawTasks, err := ReadTasksFromFile(viper.GetString("taskfile"))
		if err != nil {
			log.Log.Fatal(err)
		}

		for _, rt := range rawTasks {
			rtc <- rt
		}
	} else if cmd.Name() == "go" {
		compressedTaskSet, err := BuildCompressedTaskSet(cmd, args)
		if err != nil {
			log.Log.Fatal(err)
		}

		rawTasks := ExpandCompressedTaskSet(compressedTaskSet)
		for _, rt := range rawTasks {
			rtc <- rt
		}
	}

	// Start the process of closing up the pipeline and exit
	close(rtc)
}
