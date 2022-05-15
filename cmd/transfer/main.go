package main

import (
	"dht-ocean/cmd/transfer/types"
	"dht-ocean/dao"
	"dht-ocean/model"
	"github.com/kamva/mgm/v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

type transfer struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

func main() {
	data, err := os.ReadFile("transfer.yaml")
	if err != nil {
		panic(err)
	}
	cfg := transfer{}
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		panic(err)
	}

	fromDB, err := dao.InitDB(cfg.From)
	if err != nil {
		panic(err)
	}
	err = dao.InitMongo("dht_ocean", cfg.To)
	if err != nil {
		panic(err)
	}

	cnt := 0
	go func() {
		rows, err := fromDB.Table("torrents").Rows()
		if err != nil {
			panic(err)
		}
		defer rows.Close()
		for rows.Next() {
			record := types.AlphaReign{}
			err := fromDB.ScanRows(rows, &record)
			if err != nil {
				panic(err)
			}
			o := &model.Torrent{}
			coll := mgm.Coll(o)
			err = coll.FindByID(record.InfoHash, o)
			if err == nil {
				continue
			}

			n := record.ToTorrent()
			err = coll.Create(n)
			if err != nil {
				panic(err)
			}
			cnt += 1
		}
	}()
	for {
		logrus.Infof("Transfered %d records.", cnt)
		time.Sleep(time.Second)
	}
}
