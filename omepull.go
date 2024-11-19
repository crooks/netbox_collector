package main

import (
	"database/sql"
    "errors"
	"fmt"
	"time"

	"github.com/Masterminds/log-go"
	"github.com/crooks/netbox_collector/omeapi"
	_ "github.com/lib/pq"
	"github.com/tidwall/gjson"
)

const (
	sqlDateTime = "2006-01-02 15:04:05"
)

var (
    errUnequalInts = errors.New("unequal integers")
)

func paginate() {
	top := cfg.OmeApi.Page
	skip := 0
	count := 0
	db := dbInit()
	defer db.Close()
	testMode := false
	bypassApi := false
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
				fieldDST := v.Get("DeviceServiceTag")
				if !fieldDST.Exists() {
					log.Warn("Ignoring device without Service Tag")
					continue
				}
				dev := new(deviceFields)
				dev.deviceParser(v)
				//dev.dbDelete(db)
				//dev.dbInsert(db)
                // Type 1000 appears to indicate a server
                fieldType := v.Get("Type")
                if !fieldType.Exists() || fieldType.Int() != 1000 {
                    continue
                }
                fmt.Println(fieldDST.String())
                fieldCST := v.Get("ChassisServiceTag")
                if !fieldCST.Exists() || fieldDST.String() == fieldCST.String() {
                    fmt.Printf("%s: Not chassis hosted\n", fieldDST.String())
                    continue
                }
                fieldID := v.Get("InventoryDetails@odata\\.navigationLink").String()
                dev.deviceDetail(api, fieldID)
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
    serverSockets     int
    serverCores       int
    serverSpeed       int
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

func (dev *deviceFields) deviceDetail(api *omeapi.AuthClient, device_id string) {
	device_id_url := cfg.OmeApi.Url + device_id
	b, err := api.GetJSON(device_id_url)
	if err != nil {
		log.Fatalf("Unable to retrieve %s: %v", device_id_url, err)
	}
	gj := gjson.ParseBytes(b)
	for _, v := range gj.Get("value").Array() {
		switch v.Get("InventoryType").Str {
            case "serverProcessors":
                fmt.Printf("Device Tag: %s\n", dev.deviceServiceTag)
                fmt.Printf("Chassis Tag: %s\n", dev.chassisServiceTag)
                dev.deviceProcessors(v.Get("InventoryInfo"))
                fmt.Printf("Sockets: %d\n", dev.serverSockets)
                fmt.Printf("Cores: %d\n", dev.serverCores)
                fmt.Printf("Speed: %d\n", dev.serverSpeed)
        }
	}
}

func (dev *deviceFields) deviceProcessors(gj gjson.Result) {
    sockets := gj.Get("#").Int()
    cores := gj.Get("0.NumberOfCores").Int()
    speed := gj.Get("0.CurrentSpeed").Int()

    // All of our servers should have matching CPUs.  Throw a panic if they appear to be different.
    for _, v := range gj.Array() {
        vCores:= v.Get("NumberOfCores").Int()
        if cores != vCores {
            panic(errUnequalInts)
        }
        vSpeed := v.Get("CurrentSpeed").Int()
        if speed != vSpeed {
            panic(errUnequalInts)
        }
    }
    dev.serverSockets = int(sockets)
    dev.serverCores = int(cores)
    dev.serverSpeed = int(speed)
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
      slot_name TEXT,
      last_seen TIMESTAMP
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
	INSERT INTO assets (device_service_tag, chassis_service_tag, model, network_address, mac_address,
    dns_name, slot_number, slot_name, last_seen)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
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
		sqlTimestamp(),
	)
	if err != nil {
		panic(err)
	}
}

func (d *deviceFields) dbDelete(db *sql.DB) {
	sqlStatement := " DELETE FROM assets WHERE device_service_tag = $1"
	_, err := db.Exec(sqlStatement, d.deviceServiceTag)
	if err != nil {
		panic(err)
	}
}

func sqlTimestamp() string {
	utc := time.Now().UTC()
	return utc.Format(sqlDateTime)
}
