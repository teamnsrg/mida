package storage

import (
	"context"
	"errors"
	"github.com/spf13/viper"
	"github.com/teamnsrg/mida/log"
	t "github.com/teamnsrg/mida/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"time"
)

type objIdCounter struct {
	ID    int64  `bson:"_id"`
	Type  string `bson:"type"`
	Count int64  `bson:"count"`
}

type MongoConn struct {
	Ctx    context.Context
	Client *mongo.Client
	Coll   *mongo.Collection
	Cancel context.CancelFunc
}

func CreateMongoDBConnection(uri string, collection string) (*MongoConn, error) {
	mc := new(MongoConn)
	ctx, cancel := context.WithTimeout(context.Background(), MongoStorageTimeoutSeconds*time.Second)

	mc.Ctx = ctx
	mc.Cancel = cancel

	opts := options.Client()
	opts.Auth = &options.Credential{
		AuthMechanism:           "",
		AuthMechanismProperties: nil,
		AuthSource:              viper.GetString("mongodatabase"),
		Username:                viper.GetString("mongouser"),
		Password:                viper.GetString("mongopass"),
		PasswordSet:             false,
	}

	client, err := mongo.Connect(ctx, opts.ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	mc.Client = client
	mc.Coll = mc.Client.Database(viper.GetString("mongodatabase")).Collection(collection)

	// Make sure out default indices for the collection are in place
	indexOpts := options.CreateIndexes().SetMaxTime(600 * time.Second)
	keys := bsonx.Doc{{Key: "type", Value: bsonx.Int32(1)}}
	index1 := mongo.IndexModel{}
	index1.Keys = keys

	keys = bsonx.Doc{{Key: "callclass", Value: bsonx.Int32(1)}}
	index2 := mongo.IndexModel{}
	index2.Keys = keys

	keys = bsonx.Doc{{Key: "callfunc", Value: bsonx.Int32(1)}}
	index3 := mongo.IndexModel{}
	index3.Keys = keys

	log.Log.Debug("Ensuring Indices")
	_, err = mc.Coll.Indexes().CreateMany(mc.Ctx, []mongo.IndexModel{index1, index2, index3}, indexOpts)
	if err != nil {
		log.Log.Error(err)
	}
	log.Log.Debug("Done ensuring indices")

	return mc, nil
}

func (conn *MongoConn) TeardownConnection() error {

	err := conn.Client.Disconnect(conn.Ctx)
	if err != nil {
		return err
	}

	conn.Cancel()

	return nil
}

// Reserves a set of object IDs in the MongoDB database. Start and end are inclusive.
func (conn *MongoConn) ReserveObjIDs(num int64) (start int64, end int64, err error) {
	var updateOpts = &options.FindOneAndUpdateOptions{
		ReturnDocument: new(options.ReturnDocument),
		Upsert:         new(bool),
	}
	updateOpts.ReturnDocument = new(options.ReturnDocument)
	*updateOpts.ReturnDocument = options.After
	*updateOpts.Upsert = true

	// We set our document IDs in MongoDB manually by atomically updating a document ID counter (below)
	doc := conn.Coll.FindOneAndUpdate(
		context.Background(),
		bson.M{"_id": MaxInt64, // Unique ID for counter
			"type": "ObjIdCounter"},
		bson.M{"$inc": bson.M{"count": num}},
		updateOpts,
	)

	var counter objIdCounter
	err = doc.Decode(&counter)
	if err != nil && err.Error() != "mongo: no documents in result" {
		log.Log.Error(err)
		return -1, -1, err
	} else {
		return counter.Count + 1 - num, counter.Count, nil
	}
}

func (conn *MongoConn) StoreMetadata(r *t.FinalMIDAResult) (int64, error) {
	// Reserve an object ID
	curId, maxId, err := conn.ReserveObjIDs(1)
	if err != nil || curId != maxId {
		return -1, err
	}

	r.Metadata.ID = curId
	r.Metadata.Type = "Metadata"

	result, err := conn.Coll.InsertOne(conn.Ctx, r.Metadata)
	if err != nil {
		return -1, err
	}

	// Double check the result
	if returnedId, ok := result.InsertedID.(int64); !ok {
		return -1, errors.New("failed to decode returned object ID to int64")
	} else if returnedId != curId {
		log.Log.Error("Returned: ", returnedId)
		log.Log.Error("curId: ", curId)
		return -1, errors.New("returned object ID did not match expected")
	}

	return curId, nil
}

func (conn *MongoConn) StoreResources(r *t.FinalMIDAResult) (*[]int64, error) {
	// Reserve object IDs
	curId, maxId, err := conn.ReserveObjIDs(int64(len(r.ResourceMetadata)))
	if err != nil {
		return nil, err
	}

	toStore := make([]interface{}, 0)
	objIds := make([]int64, 0)

	// Assign object IDs and store
	for _, resource := range r.ResourceMetadata {
		resource.ID = curId
		resource.Crawl = r.Metadata.ID
		resource.Type = "Resource"
		curId++
		toStore = append(toStore, resource)
		if len(toStore) > MongoStorageResourceBufferLen {
			result, err := conn.Coll.InsertMany(conn.Ctx, toStore)
			if err != nil {
				return &objIds, err
			}
			toStore = make([]interface{}, 0)

			for _, oid := range result.InsertedIDs {
				if oint, ok := oid.(int64); !ok {
					return &objIds, errors.New("got non-int64 object id")
				} else {
					objIds = append(objIds, oint)
				}

			}

		}
	}

	if len(toStore) > 0 {
		result, err := conn.Coll.InsertMany(conn.Ctx, toStore)
		if err != nil {
			return &objIds, err
		}
		for _, oid := range result.InsertedIDs {
			if oint, ok := oid.(int64); !ok {
				return &objIds, errors.New("got non-int64 object id")
			} else {
				objIds = append(objIds, oint)
			}

		}
	}

	// Validate that we used the expected number of object IDs
	if curId != maxId+1 {
		return &objIds, errors.New("used incorrect number of object ids while storing resources to mongodb")
	}

	// Update the metadata object to include this array of resources
	result, err := conn.Coll.UpdateOne(conn.Ctx, bson.M{"_id": r.Metadata.ID}, bson.M{"$push": bson.M{"resources": bson.M{"$each": objIds}}})
	if err != nil {
		log.Log.Error(result)
		log.Log.Error(err)
	}

	return &objIds, nil
}

func (conn *MongoConn) StoreJSTrace(r *t.FinalMIDAResult) error {

	// Count the number of object IDs we will need to store the trace
	// Assign IDs indexed from zero
	var objIdAlloc int64 = 1
	for isolateID := range r.JSTrace.Isolates {
		objIdAlloc += 1
		for _, script := range r.JSTrace.Isolates[isolateID].Scripts {
			objIdAlloc += 1
			for range script.Calls {
				objIdAlloc += 1
			}
		}
	}

	curId, maxId, err := conn.ReserveObjIDs(objIdAlloc)
	if err != nil {
		return err
	}

	// Set object ID for trace
	r.JSTrace.ID = curId
	curId += 1

	// Now iterate through our trace once more and create documents to store
	for isolateID := range r.JSTrace.Isolates {
		r.JSTrace.Isolates[isolateID].ID = curId
		r.JSTrace.Children = make([]int64, 0)
		curId += 1
		for _, script := range r.JSTrace.Isolates[isolateID].Scripts {
			script.ID = curId
			script.Parent = r.JSTrace.Isolates[isolateID].ID
			script.Children = make([]int64, 0)
			curId += 1
			for j := range script.Calls {
				script.Calls[j].ID = curId
				curId += 1
			}
		}
	}

	if curId != maxId+1 {
		log.Log.Error("Current ID after allocation: ", curId)
		log.Log.Error("Max ID (Should be 1 less than current): ", maxId)
		return errors.New("current ID and max ID not properly matching up")
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

			var calls []int64
			for _, call := range script.Calls {
				calls = append(calls, call.ID)
			}

			toStore = append(toStore, &bson.M{
				"_id":             script.ID,
				"type":            "Script",
				"baseUrl":         script.BaseUrl,
				"scriptId":        script.ScriptId,
				"parent":          isolate.ID,
				"children":        calls,
				"openwpm_results": script.OpenWPM,
			})

			for _, call := range script.Calls {
				toStore = append(toStore, &bson.M{
					"_id":       call.ID,
					"type":      "Call",
					"calltype":  call.T,
					"callclass": call.C,
					"callfunc":  call.F,
					"args":      call.Args,
					"ret":       call.Ret,
					"parent":    script.ID,
					"children":  nil,
				})
				if len(toStore) > MongoStorageJSBufferLen {
					_, err := conn.Coll.InsertMany(conn.Ctx, toStore)
					if err != nil {
						return err
					}
					toStore = make([]interface{}, 0)
				}
			}
		}
	}

	// Clean up any left over documents needing to be stored
	if len(toStore) > 0 {
		_, err = conn.Coll.InsertMany(conn.Ctx, toStore)
		if err != nil {
			return err
		}
	}

	return nil
}

func (conn *MongoConn) StoreWebSocketData(r *t.FinalMIDAResult) (*[]int64, error) {
	// Reserve object IDs
	curId, maxId, err := conn.ReserveObjIDs(int64(len(r.WebsocketData)))
	if err != nil {
		return nil, err
	}

	toStore := make([]interface{}, 0)
	objIds := make([]int64, 0)

	// Assign object IDs and store
	for _, wsd := range r.WebsocketData {
		wsd.ID = curId
		wsd.Crawl = r.Metadata.ID
		wsd.Type = "Websocket"
		curId++
		toStore = append(toStore, wsd)
		if len(toStore) > MongoStorageResourceBufferLen {
			result, err := conn.Coll.InsertMany(conn.Ctx, toStore)
			if err != nil {
				return &objIds, err
			}
			toStore = make([]interface{}, 0)

			for _, oid := range result.InsertedIDs {
				if oint, ok := oid.(int64); !ok {
					return &objIds, errors.New("got non-int64 object id")
				} else {
					objIds = append(objIds, oint)
				}

			}

		}
	}

	if len(toStore) > 0 {
		result, err := conn.Coll.InsertMany(conn.Ctx, toStore)
		if err != nil {
			return &objIds, err
		}
		for _, oid := range result.InsertedIDs {
			if oint, ok := oid.(int64); !ok {
				return &objIds, errors.New("got non-int64 object id")
			} else {
				objIds = append(objIds, oint)
			}

		}
	}

	// Validate that we used the expected number of object IDs
	if curId != maxId+1 {
		return &objIds, errors.New("used incorrect number of object ids while storing resources to mongodb")
	}

	return &objIds, nil
}
