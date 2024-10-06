package main

import (
	"fmt"

	"github.com/Masterminds/log-go"
	"github.com/crooks/netbox_collector/omeapi"
	"github.com/tidwall/gjson"
)

type paginator struct {
	api   *omeapi.AuthClient
	top   int
	skip  int
	count int
}

func paginate() {
	testMode := true
	p := new(paginator)
	p.top = cfg.OmeApi.Page
	p.skip = 0
	p.count = 0
	p.api = omeapi.NewBasicAuthClient(cfg.OmeApi.UserID, cfg.OmeApi.Password, cfg.OmeApi.CertFile)
	for {
		p.omeDevices()
		if p.skip > p.count || testMode {
			break
		}
	}
}

func (p *paginator) omeDevices() {
	url := fmt.Sprintf("%s/api/DeviceService/Devices?$top=%d&skip=%d", cfg.OmeApi.Url, p.top, p.skip)
	b, err := p.api.GetJSON(url)
	if err != nil {
		log.Fatalf("Unable to retrieve %s: %v", url, err)
	}
	gj := gjson.ParseBytes(b)
	for _, v := range gj.Get("value").Array() {
		fmt.Println("----------")
		// The Device Service Tag is our unique identifier.  If it doesn't exist, ignore the record.
		dst_field := v.Get("DeviceServiceTag")
		if !dst_field.Exists() {
			log.Warn("Ignoring device without Service Tag")
			continue
		}
		printIfExists(v, "DeviceServiceTag")
		printIfExists(v, "ChassisServiceTag")
		printIfExists(v, "Model")
		printIfExists(v, "Type")
		// For our purposes, we only want the first interface in the list
		device_field := v.Get("DeviceManagement.0")
		printIfExists(device_field, "NetworkAddress")
		printIfExists(device_field, "MacAddress")
		printIfExists(device_field, "DeviceManagement.0.DnsName")
		slot_field := v.Get("SlotConfiguration")
		if slot_field.Exists() {
			printIfExists(slot_field, "SlotNumber")
			printIfExists(slot_field, "SlotName")
		}
		/*
			device_id_field := v.Get("@odata\\.id")
			if device_id_field.Exists() {
				p.omeDeviceDetail(device_id_field.String())
			}
		*/
	}
	if p.count == 0 {
		// The good people at Dell have used a . in a field name.  This needs to be \\ escaped.
		count_field := gj.Get("@odata\\.count")
		if !count_field.Exists() {
			log.Fatalf("Unable to determine record count from URL: %s", url)
		}
		p.count = int(count_field.Int())
		log.Debugf("Total record count: %d", p.count)
	}
	// top is the number of records we're fetching.  skip is the record number to start at.
	p.skip += p.top
}

func printIfExists(gj gjson.Result, key string) {
	key_field := gj.Get(key)
	if key_field.Exists() {
		fmt.Printf("%s: %s\n", key, key_field.String())
	}
}
func (p *paginator) omeDeviceDetail(device_id string) {
	device_id_url := cfg.OmeApi.Url + device_id + "/InventoryDetails('serverProcessors')"
	b, err := p.api.GetJSON(device_id_url)
	if err != nil {
		log.Fatalf("Unable to retrieve %s: %v", device_id_url, err)
	}
	gj := gjson.ParseBytes(b)
	fmt.Println(gj)
	for k, v := range gj.Get("InventoryInfo").Array() {
		fmt.Println(k, v.Get("ModelName").String())
	}
}
