package storage

import (
	"context"
	"github.com/pmurley/mida/log"
	t "github.com/pmurley/mida/types"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson"
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
	log.Log.Info(viper.GetString("mongouser"))
	log.Log.Info(viper.GetString("mongopass"))
	log.Log.Info(viper.GetString("mongodatabase"))

	client, err := mongo.Connect(ctx, opts.ApplyURI("mongodb://mongo.mida.sprai.org:27017"))
	if err != nil {
		return err
	}

	log.Log.Info("Client: ", client)

	collection := client.Database(viper.GetString("mongodatabase")).Collection(r.SanitizedTask.GroupID)

	var updateOpts = &options.FindOneAndUpdateOptions{
		ArrayFilters:             nil,
		BypassDocumentValidation: nil,
		Collation:                nil,
		MaxTime:                  nil,
		Projection:               nil,
		ReturnDocument:           nil,
		Sort:                     nil,
		Upsert:                   new(bool),
	}

	updateOpts.ReturnDocument = new(options.ReturnDocument)
	*updateOpts.ReturnDocument = 1
	*updateOpts.Upsert = true

	doc := collection.FindOneAndUpdate(
		context.Background(),
		bson.M{
			"name": "Counter",
		},
		bson.M{
			"$inc": bson.M{"val": 5},
		},
		updateOpts)
	log.Log.Info(doc)

	// Allocate object IDs for this trace

	log.Log.Info("Collection: ", collection)
	/*
		isolates := make([]interface{},0)
		for isolateID := range r.JSTrace.Isolates {
			isolates = append(isolates, bson.M{
				"type": "isolate",
				"isolateID": isolateID,
			})
		}
		log.Log.Info("Isolates:", isolates)
	*/
	isolateIDs, err := collection.InsertOne(ctx, bson.M{
		"type": "fake",
	})
	if err != nil {
		log.Log.Error(err)
	}
	log.Log.Info("Isolate IDs: ", isolateIDs)

	/*
		scripts := make([]interface{},0)
		for _, isolateID := range isolateIDs.InsertedIDs {
			for scriptId, script := range r.JSTrace.Isolates[isolateID.(string)].Scripts {
				scripts = append(scripts, bson.M{
					"type":    "script",
					"isolate": isolateID,
					"scriptId": scriptId,
				})
			}

			}
		}
	*/

	return nil
}
