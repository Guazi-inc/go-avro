package avro

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

const subject = "this_is_a_test_schema"
const rawSchema = `{"type": "record", "name": "TestRecord", "fields": [
	{"name": "longRecordField", "type": "long"},
	{"name": "floatRecordField", "type": "float"}
]}`

const rawSchema1 = `{"type": "record", "name": "TestRecord", "fields": [
{"name": "stringRecordField", "type": "string"},
{"name": "floatRecordField", "type": "float"}
]}`

const rawSchema2 = `{"type": "record", "name": "TestRecord", "fields": [
	{"name": "floatRecordField", "type": "float"}
]}`
const rawSchema3 = `{"type": "record", "name": "TestRecord", "fields": [
{"name": "intRecordField", "type": "int"},
{"name": "floatRecordField", "type": "float"}
]}`
const rawSchema4 = `{"type": "record", "name": "TestRecord", "fields": [
{"name": "floatRecordField", "type": "float"},
{"name": "floatRecordField", "type": "float"}
]}`

var regClient *CachedSchemaRegistryClient
var schema Schema
var id int32 = 735
var rawList = []string{rawSchema, rawSchema1, rawSchema2, rawSchema3, rawSchema4}
var schemaList []Schema

func init() {
	regClient = NewCachedSchemaRegistryClient("")
	_ = regClient
	for i := range rawList {
		sch, err := ParseSchema(rawList[i])
		if err != nil {
			panic(err)
		}
		schemaList = append(schemaList, sch)
	}

	_ = schema
}

// func TestRegister(t *testing.T) {
// 	if regClient == nil {
// 		t.Error("regclient is nil")
// 	}
// 	t.Errorf("regclient:%v", regClient)
// 	for i := 0; i < len(schemaList); i++ {
// 		id, err := regClient.Register(fmt.Sprintf("this_is_a_test_schema_%d", i), schemaList[i])
// 		assert(t, err, nil)
// 		t.Logf("this_is_a_test_schema_%v:id:%v", i, id)
// 	}
// }

func TestGetByID(t *testing.T) {

	now := time.Now()
	sch, err := regClient.GetByID(id)
	assert(t, err, nil)
	t.Logf("getByID scehma:%v", sch)
	for i := 0; i < 100; i++ {
		id = 700 + rand.Int31n(40)
		t.Logf("getByID:%v", time.Since(now))
		_, err := regClient.GetByID(id)
		assert(t, err, nil)
		now = time.Now()
	}

}

func TestGetIDBySchema(t *testing.T) {

	now := time.Now()

	for i := 0; i < 20; i++ {

		idx := rand.Int31n(4)

		idtemp, err := regClient.GetIDBySchema(fmt.Sprintf("this_is_a_test_schema_%v", idx), schemaList[idx])
		assert(t, err, nil)
		t.Logf("getIDBySchema id:%v", idtemp)
		t.Logf("duration:%v", time.Since(now))
		now = time.Now()
	}
}

func TestGetVersion(t *testing.T) {

	vID, err := regClient.GetVersion("this_is_a_test_schema_0", schemaList[0])
	assert(t, err, nil)
	t.Logf("twice getVersion id:%v", vID)
	now := time.Now()

	for i := 0; i < 10; i++ {
		vID, err := regClient.GetVersion("this_is_a_test_schema_0", schemaList[0])
		assert(t, err, nil)
		t.Logf("getVersion id:%v", vID)
		t.Logf("duration:%v", time.Since(now))
		now = time.Now()
	}
}
func TestGetLatestSchemaMetadata(t *testing.T) {
	meta, err := regClient.GetLatestSchemaMetadata(subject)
	assert(t, err, nil)
	t.Logf("meta:%v", meta)
}

func BenchmarkRegister(b *testing.B) {

	var wg sync.WaitGroup
	regClient := NewCachedSchemaRegistryClient("http://g1-bdp-hdp-19.dns.guazi.com:8081")
	id := int32(735)
	schema, err := regClient.GetByID(id)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := regClient.GetIDBySchema("this_is_a_test_schema", schema)
			if err != nil {
				b.Fatal(err)
			}
			_ = id
		}()
	}
	wg.Wait()
}
