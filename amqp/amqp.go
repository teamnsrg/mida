package amqp

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/streadway/amqp"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"math/rand"
	"os"
	"strings"
	"time"
)

type ConnParams struct {
	User string
	Pass string
	Uri  string
}

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
	done    chan error
}

// LoadTasks handles loading MIDA tasks in to AMQP (probably RabbitMQ) queue.
func LoadTasks(tasks b.TaskSet, params ConnParams, queue string, priority uint8, shuffle bool) (int, error) {
	amqpUri := fullUriFromParams(params)
	log.Log.Debugf("Connecting to AMQP instance at %s", params.Uri)

	var connection *amqp.Connection
	var err error

	if strings.HasPrefix(amqpUri, "amqps") {
		connection, err = amqp.DialTLS(amqpUri,
			&tls.Config{
				InsecureSkipVerify: true,
			},
		)
	} else if strings.HasPrefix(amqpUri, "amqp") {
		connection, err = amqp.Dial(amqpUri)
	} else {
		err = errors.New("invalid amqp URL: [ " + amqpUri + " ]")
	}
	if err != nil {
		return 0, err
	}
	defer connection.Close()

	channel, err := connection.Channel()
	if err != nil {
		return 0, err
	}

	if shuffle {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(tasks),
			func(i, j int) { tasks[i], tasks[j] = tasks[j], tasks[i] })
	}

	tasksLoaded := 0
	for _, task := range tasks {
		taskBytes, err := json.Marshal(task)
		if err != nil {
			return tasksLoaded, err
		}

		err = channel.Publish(
			"",
			queue,
			false,
			false,
			amqp.Publishing{
				Headers:      amqp.Table{},
				ContentType:  "text/plain",
				DeliveryMode: 0,
				Priority:     priority,
				Timestamp:    time.Now(),
				Body:         taskBytes,
			})
		if err != nil {
			return tasksLoaded, err
		}

		// Successfully loaded a task into the queue
		tasksLoaded += 1
	}

	return tasksLoaded, nil
}

func NewAMQPTasksConsumer(params ConnParams, queue string) (*Consumer, <-chan amqp.Delivery, error) {
	c := &Consumer{
		conn:    nil,
		channel: nil,
		tag:     "",
		done:    make(chan error),
	}

	var err error
	amqpUri := fullUriFromParams(params)
	log.Log.Debugf("Connecting to AMQP instance at %s", params.Uri)

	if strings.HasPrefix(amqpUri, "amqps") {
		c.conn, err = amqp.DialTLS(amqpUri,
			&tls.Config{
				InsecureSkipVerify: true,
			},
		)
	} else if strings.HasPrefix(amqpUri, "amqp") {
		c.conn, err = amqp.Dial(amqpUri)
	} else {
		err = errors.New("invalid amqp URL: [ " + amqpUri + " ]")
	}
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
	taskQueue, err := c.channel.QueueDeclarePassive(
		queue, // name of the queue
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, nil, err
	}

	// Creates a new channel where deliveries from AMQP will arrive
	deliveryChan, err := c.channel.Consume(
		taskQueue.Name, // name
		c.tag,          // consumerTag,
		false,          // autoAck
		false,          // exclusive
		true,           // noLocal
		false,          // noWait
		nil,            // arguments
	)
	if err != nil {
		return nil, nil, err
	}

	return c, deliveryChan, nil
}

// NewAMQPControlConsumer connects to a AMQP which will be used for control messages rather than tasks
func NewAMQPBroadcastConsumer(params ConnParams, queue string) (*Consumer, <-chan amqp.Delivery, error) {
	c := &Consumer{
		conn:    nil,
		channel: nil,
		tag:     "",
		done:    make(chan error),
	}

	var err error
	amqpUri := fullUriFromParams(params)

	if strings.HasPrefix(amqpUri, "amqps") {
		c.conn, err = amqp.DialTLS(amqpUri,
			&tls.Config{
				InsecureSkipVerify: true,
			},
		)
	} else if strings.HasPrefix(amqpUri, "amqp") {
		c.conn, err = amqp.Dial(amqpUri)
	} else {
		err = errors.New("invalid amqp URL: [ " + amqpUri + " ]")
	}
	if err != nil {
		return nil, nil, err
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, nil, err
	}

	// Create a name for our host-specific queue, which will be attached to the broadcast
	// exchange so we can receive broadcast messages
	h, err := os.Hostname()
	if err != nil {
		return nil, nil, err
	}
	queueName := h + "-broadcast"

	hostBroadcastQueue, err := c.channel.QueueDeclare(
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
		hostBroadcastQueue.Name,
		"",                       // binding key (ignored in fanout exchanges)
		DefaultBroadcastExchange, // Exchange
		false,                    // noWait
		nil,                      // arguments
	)
	if err != nil {
		return nil, nil, err
	}

	// Creates a new channel where deliveries from AMQP will arrive
	deliveryChan, err := c.channel.Consume(
		hostBroadcastQueue.Name, // name
		c.tag,                   // consumerTag,
		true,                    // autoAck
		false,                   // exclusive
		true,                    // noLocal
		false,                   // noWait
		nil,                     // arguments
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
func DecodeAMQPMessageToRawTask(delivery amqp.Delivery) (b.RawTask, error) {
	var task b.RawTask
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

func fullUriFromParams(params ConnParams) string {
	var amqpUri string
	if strings.HasPrefix(params.Uri, "amqp://") {
		amqpUri = "amqp://" + params.User + ":" + params.Pass + "@" + strings.Replace(params.Uri, "amqp://", "", 1)
	} else if strings.HasPrefix(params.Uri, "amqps://") {
		amqpUri = "amqps://" + params.User + ":" + params.Pass + "@" + strings.Replace(params.Uri, "amqps://", "", 1)
	} else {
		amqpUri = "amqp://" + params.User + ":" + params.Pass + "@" + params.Uri
	}
	return amqpUri
}
