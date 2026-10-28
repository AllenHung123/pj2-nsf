package db

import (
	"context"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const mongoTimeout = 5 * time.Second

type mongoDb struct {
	client     *mongo.Client
	collection *mongo.Collection
}

type mongoRecord struct {
	Key   string `bson:"_id"`
	Value string `bson:"value"`
}

var _ DbIf = (*mongoDb)(nil)

func newMongoDb() (*mongoDb, error) {
	uri := os.Getenv("NSF_MONGO_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27017"
	}
	database := os.Getenv("NSF_MONGO_DATABASE")
	if database == "" {
		database = "pj2_nsf"
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(mongoTimeout))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return &mongoDb{client: client, collection: client.Database(database).Collection("services")}, nil
}

func (m *mongoDb) Release() error {
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()
	return m.client.Disconnect(ctx)
}

func (m *mongoDb) Save(key, value string) error { return m.SaveAll(map[string]string{key: value}) }

func (m *mongoDb) SaveAll(data map[string]string) error {
	if len(data) == 0 {
		return nil
	}
	writes := make([]mongo.WriteModel, 0, len(data))
	for key, value := range data {
		writes = append(writes, mongo.NewReplaceOneModel().SetFilter(bson.M{"_id": key}).SetReplacement(mongoRecord{key, value}).SetUpsert(true))
	}
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()
	_, err := m.collection.BulkWrite(ctx, writes)
	return err
}

func (m *mongoDb) Exist(key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()
	count, err := m.collection.CountDocuments(ctx, bson.M{"_id": key}, options.Count().SetLimit(1))
	return count > 0, err
}

func (m *mongoDb) Load(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()
	var record mongoRecord
	err := m.collection.FindOne(ctx, bson.M{"_id": key}).Decode(&record)
	return record.Value, err
}

func (m *mongoDb) LoadAll() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()
	cursor, err := m.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var records []mongoRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, err
	}
	data := make(map[string]string, len(records))
	for _, record := range records {
		data[record.Key] = record.Value
	}
	return data, nil
}

func (m *mongoDb) Remove(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()
	_, err := m.collection.DeleteOne(ctx, bson.M{"_id": key})
	return err
}

func (m *mongoDb) RemoveAll() error {
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()
	_, err := m.collection.DeleteMany(ctx, bson.M{})
	return err
}
