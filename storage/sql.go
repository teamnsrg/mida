package storage

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/spf13/viper"
	"github.com/teamnsrg/mida/jstrace"
	"github.com/teamnsrg/mida/log"
	"time"
)

type SqlCall struct {
	gorm.Model

	CrawlID   int `gorm:"not null"`
	IsolateID int `gorm:"not null"`
	ScriptID  int `gorm:"not null"`
	CallID    int `gorm:"not null"`
	SeqNum    int `gorm:"not null"`
}

type SqlScript struct {
	CrawlId   int    `gorm:"not null"`
	IsolateId int    `gorm:"not null"`
	ScriptId  int    `gorm:"not null"`
	Bytes     int    `gorm:"not null"`
	Calls     int    `gorm:"not null"`
	Url       string `gorm:"not null"`
	Sha256    string `gorm:"not null"`
}

type SqlMetadata struct {
	CrawlId  int       `gorm:"not null"`
	TS       time.Time `gorm:"not null"`
	Url      string    `gorm:"not null"`
	RandomId string    `gorm:"not null"`
	Failed   bool      `gorm:"not null"`
}

// CreatePostgresConnection connects to the postgres server and creates the specified database, if it does not
// already exist.
func CreatePostgresConnection(host string, port string, dbName string) (*gorm.DB, error) {

	log.Log.Info("Attempting connection:")
	log.Log.Infof("Host: [ %s ]", host)
	log.Log.Infof("Port: [ %s ]", port)
	log.Log.Infof("DB: [ %s ]", dbName)
	log.Log.Infof("Username: [ %s ]", viper.GetString("postgresuser"))
	log.Log.Infof("Password: [ %s ]", viper.GetString("postgrespass"))

	db, err := gorm.Open("postgres",
		"host="+host+
			" port="+port+
			" user="+viper.GetString("postgresuser")+
			" dbname="+"postgres"+
			" password="+viper.GetString("postgrespass"))
	if err != nil {
		return nil, err
	}

	// Send the logs from gorm into our own logging infrastructure
	db.SetLogger(log.Log)

	// This will error if the database already exists. That's okay - we are going to connect to it anyway
	db = db.Exec("CREATE DATABASE " + dbName + " WITH TEMPLATE mida_template OWNER mida;")

	db, err = gorm.Open("postgres",
		"host="+host+
			" port="+port+
			" user="+viper.GetString("postgresuser")+
			" dbname="+dbName+
			" password="+viper.GetString("postgrespass"))
	if err != nil {
		return nil, err
	}

	return db, nil

}

func StoreJSTraceToDB(db *gorm.DB, trace *jstrace.JSTrace) error {

	//callStmt := `INSERT INTO calls(crawl_id,isolate_id,script_id,call_id,seq_num,args) VALUES %s`

	for _, iso := range trace.Isolates {
		for _, scr := range iso.Scripts {
			for _, _ = range scr.Calls {
				log.Log.Debug("hello")

			}
		}
	}

	return nil
}
