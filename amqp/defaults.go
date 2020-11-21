package amqp

const (
	DefaultTls               = false // Whether to use TLS connection by default
	DefaultPort              = 5672
	DefaultPriority          = 5 // Default priority of tasks loaded into queue
	DefaultTaskQueue         = "mida-tasks"
	DefaultBroadcastExchange = "mida-broadcast"
	DefaultPostQueue         = "mida-complete"
)
