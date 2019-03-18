package storage

import (
	"context"
	"github.com/pmurley/mida/log"
	t "github.com/pmurley/mida/types"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

func MongoStoreJSTrace(r *t.FinalMIDAResult) error {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	opts := options.Client()
	opts.Auth = &options.Credential{
		AuthMechanism:           "",
		AuthMechanismProperties: nil,
		AuthSource:              viper.GetString("mongodatabase"),
		Username:                viper.GetString("mongouser"),
		Password:                viper.GetString("mongopass"),
		PasswordSet:             false,
	}

	client, err := mongo.Connect(ctx, opts.ApplyURI("mongodb://mongo.mida.sprai.org:27017"))
	if err != nil {
		return err
	}

	collection := client.Database(viper.GetString("mongodatabase")).Collection(r.SanitizedTask.GroupID)

	log.Log.Info(collection)
	/*
		isolates := make([]interface{},0)
		for isolateID := range r.JSTrace.Isolates {
			isolates = append(isolates, bson.M{
				"type": "isolate",
				"isolateID": isolateID,
			})
		}
		isolateIDs, err := collection.InsertMany(ctx,isolates)

		scripts := make([]interface{},0)
		for _, isolateID := range isolateIDs.InsertedIDs {
			for scriptId, script := range r.JSTrace.Isolates[isolateID.(string)].Scripts {
				scripts = append(scripts, bson.M{
					"type": "script",
					"isolate": isolateID,
					"scriptId":
				})


			}
		}

	*/

	return nil
}
