module github.com/teamnsrg/mida

go 1.15

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/chromedp/cdproto v0.0.0-20211002082225-0242b9dca9f4
	github.com/google/uuid v1.3.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/montanaflynn/stats v0.6.6
	github.com/pkg/sftp v1.13.4
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.31.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/snowzach/rotatefilehook v0.0.0-20180327172521-2f64f265f58c
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.9.0
	github.com/streadway/amqp v1.0.0
	github.com/teamnsrg/chromedp v0.5.4-0.20210813203321-d7eb756e0987
	github.com/teamnsrg/profparse v0.0.0-20210816141139-235e259c6843
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/sys v0.0.0-20211004093028-2c5d950f24ef // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
)

// replace github.com/teamnsrg/chromedp => ../chromedp
