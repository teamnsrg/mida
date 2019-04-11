package queue

import (
	"encoding/json"
	"github.com/spf13/viper"
	"github.com/streadway/amqp"
	"github.com/teamnsrg/mida/log"
	t "github.com/teamnsrg/mida/types"
	"github.com/teamnsrg/mida/util"
	"os"
)

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
	done    chan error
}

// Takes an array of MIDATasks and loads them into a RabbitMQ queue
// Requires the RabbitMQ URI (along with valid credentials)
func AMQPLoadTasks(tasks []t.MIDATask) (int, error) {
	tasksLoaded := 0

	// Build our URI, including creds. User and pass can be set with, in order
	// of precedence:
	// 1. Command Line flag
	// 2. Environment variables
	// 3. Config file
	rabbitURI := "amqp://" + viper.GetString("rabbitmquser") + ":" +
		viper.GetString("rabbitmqpass") + "@" + viper.GetString("rabbitmqurl")

	// TODO: TLS pls
	connection, err := amqp.Dial(rabbitURI)
	if err != nil {
		log.Log.Fatal(err)
	}
	defer connection.Close()

	channel, err := connection.Channel()
	if err != nil {
		log.Log.Error(err)
		return tasksLoaded, err
	}

	for _, task := range tasks {
		taskBytes, err := json.Marshal(task)
		if err != nil {
			return tasksLoaded, err
		}

		var priority uint8
		priority = 5 // Default task priority (Possible values are 1-10 inclusiive)
		if task.Priority != nil && *task.Priority != 0 {
			priority = uint8(*task.Priority)
		}
		if priority < 1 || priority > 10 {
			log.Log.Warnf("Got bad priority for task: %d", *task.Priority)
			log.Log.Warn("Setting priority to 5")
			priority = 5
		}

		err = channel.Publish(
			"",      // Exchange
			"tasks", // Key (queue)
			false,   // Mandatory
			false,   // Immediate
			amqp.Publishing{
				Headers:         amqp.Table{},
				ContentType:     "text/plain",
				ContentEncoding: "",
				DeliveryMode:    0,
				Priority:        priority,
				Body:            taskBytes,
			})
		if err != nil {
			log.Log.Error(err)
			return tasksLoaded, err
		} else {
			tasksLoaded += 1
		}
	}

	return tasksLoaded, nil
}

func NewAMQPTasksConsumer() (*Consumer, <-chan amqp.Delivery, error) {
	c := &Consumer{
		conn:    nil,
		channel: nil,
		tag:     "",
		done:    make(chan error),
	}

	var err error
	// Build our URI, including creds. User and pass can be set with, in order
	// of precedence:
	// 1. Command Line flag
	// 2. Environment variables
	// 3. Config file
	rabbitURI := "amqp://" + viper.GetString("rabbitmquser") + ":" +
		viper.GetString("rabbitmqpass") + "@" + viper.GetString("rabbitmqurl")

	c.conn, err = amqp.Dial(rabbitURI)
	if err != nil {
		return nil, nil, err
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, nil, err
	}

	// Set this so that the queue will release the next task
	// to any available consumer
	err = c.channel.Qos(1, 0, true)
	if err != nil {
		return nil, nil, err
	}

	// Check to make sure this queue is present -- If not, we just die
	queue, err := c.channel.QueueDeclarePassive(
		viper.GetString("rabbitmqtaskqueue"), // name of the queue
		true,                                 // durable
		false,                                // delete when unused
		false,                                // exclusive
		false,                                // noWait
		nil,                                  // arguments
	)
	if err != nil {
		return nil, nil, err
	}

	// Creates a new channel where deliveries from AMQP will arrive
	deliveryChan, err := c.channel.Consume(
		queue.Name, // name
		c.tag,      // consumerTag,
		false,      // autoAck
		false,      // exclusive
		true,       // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		return nil, nil, err
	}

	return c, deliveryChan, nil
}

func NewAMQPBroadcastConsumer() (*Consumer, <-chan amqp.Delivery, error) {
	c := &Consumer{
		conn:    nil,
		channel: nil,
		tag:     "",
		done:    make(chan error),
	}

	var err error
	// Build our URI, including creds. User and pass can be set with, in order
	// of precedence:
	// 1. Command Line flag
	// 2. Environment variables
	// 3. Config file
	rabbitURI := "amqp://" + viper.GetString("rabbitmquser") + ":" +
		viper.GetString("rabbitmqpass") + "@" + viper.GetString("rabbitmqurl")

	c.conn, err = amqp.Dial(rabbitURI)
	if err != nil {
		return nil, nil, err
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, nil, err
	}

	// Create a name for the transient broadcast queue we are creating
	h, err := os.Hostname()
	if err != nil {
		log.Log.Fatal(err)
	}
	queueName := h + "-" + util.GenRandomIdentifier()

	// Check to make sure this queue is present -- If not, we just die
	queue, err := c.channel.QueueDeclare(
		queueName, // name of the queue
		false,     // durable
		true,      // delete when unused
		false,     // exclusive
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return nil, nil, err
	}

	// Bind this queue to the broadcast exchange
	err = c.channel.QueueBind(
		queue.Name,
		"", //binding key (ignored in fanout exchanges)
		viper.GetString("rabbitmqbroadcastqueue"), // Exchange
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, nil, err
	}

	// Creates a new channel where deliveries from AMQP will arrive
	deliveryChan, err := c.channel.Consume(
		queue.Name, // name
		c.tag,      // consumerTag,
		true,       // autoAck
		false,      // exclusive
		true,       // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		return nil, nil, err
	}

	return c, deliveryChan, nil
}

// Graceful shutdown of a connection to AMQP
func (c *Consumer) Shutdown() error {
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return err
	}

	if err := c.conn.Close(); err != nil {
		return err
	}

	return nil
}

// Takes an AMQP message (which is expected to be a MIDATask, in JSON format)
// and converts it into an actual MIDATask struct
func DecodeAMQPMessageToRawTask(delivery amqp.Delivery) (t.MIDATask, error) {
	var task t.MIDATask
	err := json.Unmarshal(delivery.Body, &task)
	if err != nil {
		return task, err
	}

	// Acknowledge that the task was received
	err = delivery.Ack(false)
	if err != nil {
		return task, err
	}

	return task, nil
}
