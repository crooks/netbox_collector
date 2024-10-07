package main

import (
	"database/sql"
	"fmt"

	"github.com/Masterminds/log-go"
	"github.com/crooks/netbox_collector/omeapi"
	_ "github.com/lib/pq"
	"github.com/tidwall/gjson"
)

func paginate() {
	top := cfg.OmeApi.Page
	skip := 0
	count := 0
	db := dbInit()
	defer db.Close()
	testMode := true
	bypassApi := true
	if !bypassApi {
		api := omeapi.NewBasicAuthClient(cfg.OmeApi.UserID, cfg.OmeApi.Password, cfg.OmeApi.CertFile)
		for {
			url := fmt.Sprintf("%s/api/DeviceService/Devices?$top=%d&skip=%d", cfg.OmeApi.Url, top, skip)
			b, err := api.GetJSON(url)
			if err != nil {
				log.Fatalf("Unable to retrieve %s: %v", url, err)
			}
			gj := gjson.ParseBytes(b)
			for _, v := range gj.Get("value").Array() {
				// The Device Service Tag is our unique identifier.  If it doesn't exist, ignore the record.
				dst_field := v.Get("DeviceServiceTag")
				if !dst_field.Exists() {
					log.Warn("Ignoring device without Service Tag")
					continue
				}
				dev := new(deviceFields)
				dev.deviceParser(gj)
				dev.dbInsert(db)
			}
			if count == 0 {
				// The good people at Dell have used a . in a field name.  This needs to be \\ escaped.
				count_field := gj.Get("@odata\\.count")
				if !count_field.Exists() {
					log.Fatalf("Unable to determine record count from URL: %s", url)
				}
				count = int(count_field.Int())
				log.Debugf("Total record count: %d", count)
			}
			// top is the number of records we're fetching.  skip is the record number to start at.
			skip += top
			if skip > count || testMode {
				break
			}
		}
	}
}

type deviceFields struct {
	deviceServiceTag  string
	chassisServiceTag string
	model             string
	networkAddress    string
	macAddress        string
	dnsName           string
	slotNumber        int
	slotName          string
}

func (dev *deviceFields) deviceParser(gj gjson.Result) {
	dev.deviceServiceTag = gj.Get("DeviceServiceTag").String()
	dev.chassisServiceTag = gj.Get("ChassisServiceTag").String()
	dev.model = gj.Get("Model").String()
	// For our purposes, we only want the first interface in the list
	device_field := gj.Get("DeviceManagement.0")
	dev.networkAddress = device_field.Get("NetworkAddress").String()
	dev.macAddress = device_field.Get("MacAddress").String()
	dev.dnsName = device_field.Get("DnsName").String()
	slot_field := gj.Get("SlotConfiguration")
	if slot_field.Exists() {
		dev.slotNumber = int(slot_field.Get("SlotNumber").Int())
		dev.slotName = slot_field.Get("SlotName").String()
	}
}

func dbInit() *sql.DB {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DbName)
	log.Debugf("PostgreSQL Connection String: %s", psqlInfo)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		panic(err)
	}
	sqlStatement := `CREATE TABLE IF NOT EXISTS assets (
	  device_service_tag TEXT PRIMARY KEY,
	  chassis_service_tag TEXT,
	  model TEXT,
	  network_address TEXT,
	  mac_address TEXT,
	  dns_name TEXT,
	  slot_number INT,
      slot_name TEXT
	  );`
	_, err = db.Exec(sqlStatement)
	if err != nil {
		fmt.Println(sqlStatement)
		panic(err)
	}
	return db
}

func (d *deviceFields) dbInsert(db *sql.DB) {
	sqlStatement := `
	INSERT INTO assets (device_service_tag, chassis_service_tag, model, network_address, mac_address, dns_name, slot_number, slot_name)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := db.Exec(
		sqlStatement,
		d.deviceServiceTag,
		d.chassisServiceTag,
		d.model,
		d.networkAddress,
		d.macAddress,
		d.dnsName,
		d.slotNumber,
		d.slotName,
	)
	if err != nil {
		panic(err)
	}
}
