package main

import (
	"database/sql"
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

				// Type 1000 appears to indicate a server
				fieldType := v.Get("Type")
				if fieldType.Exists() && fieldType.Int() == 1000 {
					fieldID := v.Get("InventoryDetails@odata\\.navigationLink").String()
					dev.deviceDetail(api, fieldID)
				}
				if !testMode {
					dev.dbDelete(db)
					dev.dbInsert(db)
				}
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
	memoryNumDIMMS    int
	memoryDIMMSize    int
	memoryTotal       int
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
	fmt.Printf("Device Tag: %s\n", dev.deviceServiceTag)
	fmt.Printf("Chassis Tag: %s\n", dev.chassisServiceTag)
	device_id_url := cfg.OmeApi.Url + device_id
	b, err := api.GetJSON(device_id_url)
	if err != nil {
		log.Fatalf("Unable to retrieve %s: %v", device_id_url, err)
	}
	gj := gjson.ParseBytes(b)
	for _, v := range gj.Get("value").Array() {
		switch v.Get("InventoryType").Str {
		case "serverProcessors":
			dev.deviceProcessors(v.Get("InventoryInfo"))
		case "serverMemoryDevices":
			dev.deviceMemory(v.Get("InventoryInfo"))
		}
	}
}

func (dev *deviceFields) deviceProcessors(gj gjson.Result) {
	sockets := gj.Get("#").Int()
	cores := gj.Get("0.NumberOfCores").Int()
	speed := gj.Get("0.CurrentSpeed").Int()
	dev.serverSockets = int(sockets)
	dev.serverCores = int(cores)
	dev.serverSpeed = int(speed)
}

func (dev *deviceFields) deviceMemory(gj gjson.Result) {
	firstSize := gj.Get("0.Size").Int()
	firstSpeed := gj.Get("0.Speed").Int()
	var memTotal int64
	for _, v := range gj.Array() {
		dimmSize := v.Get("Size").Int()
		dimmSpeed := v.Get("Speed").Int()
		if dimmSize != firstSize {
			log.Warnf("Mismatched DIMM sizes in device: %s", dev.deviceServiceTag)
		}
		memTotal += v.Get("Size").Int()
		if dimmSpeed != firstSpeed {
			log.Warnf("Mismatch DIMM speeds in device: %s", dev.deviceServiceTag)
		}
	}
	dev.memoryTotal = int(memTotal / 1024)
	dev.memoryDIMMSize = int(firstSize)
	numDIMMS := int(gj.Get("#").Int())
	if numDIMMS%2 != 0 {
		log.Warnf("Odd number of DIMMS in device: %s", dev.deviceServiceTag)
	}
	dev.memoryNumDIMMS = numDIMMS
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
	  server_cpu_sockets INT,
	  server_cpu_cores INT,
	  server_cpu_speed INT,
	  server_dimm_num INT,
	  server_dimm_size INT,
	  server_memory INT,
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
    dns_name, slot_number, slot_name, server_cpu_sockets, server_cpu_cores, server_cpu_speed,
	server_dimm_num, server_dimm_size, server_memory, last_seen)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`
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
		d.serverSockets,
		d.serverCores,
		d.serverSpeed,
		d.memoryNumDIMMS,
		d.memoryDIMMSize,
		d.memoryTotal,
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
