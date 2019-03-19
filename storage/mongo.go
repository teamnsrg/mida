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

type objIdCounter struct {
	ID    int64  `bson:"_id"`
	Type  string `bson:"type"`
	Count int64  `bson:"count"`
}

func MongoStoreJSTrace(r *t.FinalMIDAResult) error {

	// Count the number of object IDs we will need to store the trace
	// Assign IDs indexed from zero
	objIdAlloc := 1
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

	// For now, we open a new connection to Mongo every time we store a new trace
	ctx, _ := context.WithTimeout(context.Background(), MongoStorageTimeoutSeconds*time.Second)
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

	// We set our document IDs in MongoDB manually by atomically updating a document ID counter (below)
	doc := collection.FindOneAndUpdate(
		context.Background(),
		bson.M{"_id": 9223372036854775807, // Max int64, unique ID for counter
			"type": "ObjIdCounter"},
		bson.M{"$inc": bson.M{"count": objIdAlloc}},
		updateOpts,
	)

	var counter objIdCounter
	var curId int64
	err = doc.Decode(&counter)
	if err != nil && err.Error() != "mongo: no documents in result" {
		curId = 0
	} else {
		curId = counter.Count + 1
	}

	// Set object ID for trace
	r.JSTrace.ID = curId
	curId += 1

	// Now iterate through our trace once more and create documents to store
	for isolateID := range r.JSTrace.Isolates {
		r.JSTrace.Isolates[isolateID].ID = curId
		curId += 1
		for _, script := range r.JSTrace.Isolates[isolateID].Scripts {
			script.ID = curId
			curId += 1
			for i := range script.Executions {
                script.Executions[i].ID = curId
				curId += 1
				for j := range script.Executions[i].Calls {
                    script.Executions[i].Calls[j].ID = curId
					curId += 1
				}
			}
		}
	}

	toStore := make([]interface{}, 0)
	var isolates []int64
	for _, script := range r.JSTrace.Isolates {
		isolates = append(isolates, script.ID)
	}
	toStore = append(toStore, &bson.M{
		"_id":      r.JSTrace.ID,
		"type":     "Trace",
		"parent":   nil,
		"children": isolates,
		"url":      r.SanitizedTask.Url,
	})

	for isolateID, isolate := range r.JSTrace.Isolates {
		var scripts []int64
		for _, script := range isolate.Scripts {
			scripts = append(scripts, script.ID)
		}
		toStore = append(toStore, &bson.M{
			"_id":      isolate.ID,
			"type":     "Isolate",
			"parent":   r.JSTrace.ID,
			"children": scripts,
		})
		for _, script := range r.JSTrace.Isolates[isolateID].Scripts {
			var executions []int64
			for _, execution := range script.Executions {
				executions = append(executions, execution.ID)
			}
			toStore = append(toStore, &bson.M{
				"_id":      script.ID,
				"type":     "Script",
				"baseUrl":  script.BaseUrl,
				"scriptId": script.ScriptId,
				"parent":   isolate.ID,
				"children": executions,
			})
			for _, execution := range script.Executions {
				var calls []int64
				for _, call := range execution.Calls {
					calls = append(calls, call.ID)
				}
				toStore = append(toStore, &bson.M{
					"_id":      execution.ID,
					"type":     "Execution",
					"parent":   script.ID,
					"children": calls,
				})
				for _, call := range execution.Calls {
					toStore = append(toStore, &bson.M{
						"_id":       call.ID,
						"type":      "Call",
						"calltype":  call.T,
						"callclass": call.C,
						"callfunc":  call.F,
						"args":      call.Args,
						"ret":       call.Ret,
						"parent":    execution.ID,
						"children":  nil,
					})
				}
			}

		}
	}

	_, err = collection.InsertMany(ctx, toStore)
	if err != nil {
		return err
	}

	return nil
}
