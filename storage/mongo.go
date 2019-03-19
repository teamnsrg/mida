package storage

import (
	"context"
	"github.com/pmurley/mida/log"
	t "github.com/pmurley/mida/types"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type objIdCounter struct {
	ID    primitive.ObjectID `bson:"_id"`
	Name  string             `bson:"type"`
	Count int                `bson:"count"`
}

func MongoStoreJSTrace(r *t.FinalMIDAResult) error {

	// Count the number of object IDs we will need to store the trace
	// Assign IDs indexed from zero
	objIdAlloc := 0
	for isolateID := range r.JSTrace.Isolates {
		objIdAlloc += 1
		for _, script := range r.JSTrace.Isolates[isolateID].Scripts {
			objIdAlloc += 1
			for _, execution := range script.Executions {
				objIdAlloc += 1
				for range execution.Calls {
					objIdAlloc += 1
				}
			}

		}
	}

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

	var updateOpts = &options.FindOneAndUpdateOptions{
		ReturnDocument: new(options.ReturnDocument),
		Upsert:         new(bool),
	}
	updateOpts.ReturnDocument = new(options.ReturnDocument)
	*updateOpts.ReturnDocument = options.Before
	*updateOpts.Upsert = true
	doc := collection.FindOneAndUpdate(
		context.Background(),
		bson.M{"_id": "ffffffffffffffffffffffff",
			"type": "ObjIdCounter"},
		bson.M{"$inc": bson.M{"count": objIdAlloc}},
		updateOpts,
	)

	var counter objIdCounter
	err = doc.Decode(&counter)
	if err != nil {
		return err
	}
	// It is fine if it didn't return a counter for the collection.
	// We just create one
	if doc.Err() != nil && doc.Err().Error() != "mongo: no documents in result" {
		log.Log.Error(">", doc.Err().Error(), "<")
		return err
	}

	log.Log.Info(counter.Name, counter.Count)

	return nil
}
