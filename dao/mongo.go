package dao

import (
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func InitMongo(dbName, uri string) error {
	return mgm.SetDefaultConfig(nil, dbName, options.Client().ApplyURI(uri))
}
