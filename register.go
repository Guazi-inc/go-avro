package avro

/**
 *  Simple schema client for goavro,
 *  wrap for goavro/schemaclient, added a client interface for unit test
 */
import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"github.com/getsentry/raven-go"
)

const (
	GET_SCHEMA_BY_ID             = "/schemas/ids/%d"
	GET_SUBJECTS                 = "/subjects"
	GET_SUBJECT_VERSIONS         = "/subjects/%s-value/versions"
	GET_SPECIFIC_SUBJECT_VERSION = "/subjects/%s-value/versions/%s"
	REGISTER_NEW_SCHEMA          = "/subjects/%s-value/versions"
	CHECK_IS_REGISTERED          = "/subjects/%s-value"
	TEST_COMPATIBILITY           = "/compatibility/subjects/%s-value/versions/%s"
	CONFIG                       = "/config"
	ENV_REGISTRY                 = "SCHEMA_REGISTRY_ADDR"
)

type SchemaRegistryClient interface {
	Register(subject string, schema Schema) (int32, error)
	GetByID(id int32) (Schema, error)
	GetLatestSchemaMetadata(subject string) (*SchemaMetadata, error)
	GetVersion(subject string, schema Schema) (int32, error)
	GetIDBySchema(subject string, schema Schema) (int32, error)
}

type RegistryAuth struct {
	User string
	Key  string
}

func NewRegistryAuth(user string, key string) *RegistryAuth {
	return &RegistryAuth{
		User: user,
		Key:  key,
	}
}

type SchemaMetadata struct {
	Id      int32
	Version int32
	Schema  string
}

type CompatibilityLevel string

const (
	BackwardCompatibilityLevel CompatibilityLevel = "BACKWARD"
	ForwardCompatibilityLevel  CompatibilityLevel = "FORWARD"
	FullCompatibilityLevel     CompatibilityLevel = "FULL"
	NoneCompatibilityLevel     CompatibilityLevel = "NONE"
)

const (
	SCHEMA_REGISTRY_V1_JSON               = "application/vnd.schemaregistry.v1+json"
	SCHEMA_REGISTRY_V1_JSON_WEIGHTED      = "application/vnd.schemaregistry.v1+json"
	SCHEMA_REGISTRY_MOST_SPECIFIC_DEFAULT = "application/vnd.schemaregistry.v1+json"
	SCHEMA_REGISTRY_DEFAULT_JSON          = "application/vnd.schemaregistry+json"
	SCHEMA_REGISTRY_DEFAULT_JSON_WEIGHTED = "application/vnd.schemaregistry+json qs=0.9"
	JSON                                  = "application/json"
	JSON_WEIGHTED                         = "application/json qs=0.5"
	GENERIC_REQUEST                       = "application/octet-stream"
)

const (
	SCHEMA_NOT_FOUND int32 = 40403
)

var PREFERRED_RESPONSE_TYPES = []string{SCHEMA_REGISTRY_V1_JSON, SCHEMA_REGISTRY_DEFAULT_JSON, JSON}

type ErrorMessage struct {
	Error_code int32
	Message    string
}

func (this *ErrorMessage) Error() string {
	return fmt.Sprintf("%s(error code: %d)", this.Message, this.Error_code)
}

type RegisterSchemaResponse struct {
	Id int32
}

type GetSchemaResponse struct {
	Schema string
}

type GetSubjectVersionResponse struct {
	Subject string
	Version int32
	Id      int32
	Schema  string
}

type CachedSchemaRegistryClient struct {
	registryURL string

	schemaCache  sync.Map
	idCache      sync.Map
	versionCache sync.Map
	auth         *RegistryAuth
	isReg        bool
}

/**
 *  Refactor to interface, for Unit testing
 */
type RegistryClient interface {
	Register(subject string, schema Schema) (int32, error)
	GetByID(id int32) (Schema, error)
	GetIDBySchema(subject string, schema Schema) (int32, error)
	IsReg() bool
}

func NewCachedSchemaRegistryClient(registryURL string) *CachedSchemaRegistryClient {
	if registryURL == "" {
		registryURL = os.Getenv(ENV_REGISTRY)
	}
	fmt.Println("RegistryURL=", registryURL)
	if registryURL == "" {
		fmt.Println("Warning: you have not set SCHEMA_REGISTRY_ADDR in the env, will not register the schema!")
	}
	return NewCachedSchemaRegistryClientAuth(registryURL, nil)
}

func NewCachedSchemaRegistryClientAuth(registryURL string, auth *RegistryAuth) *CachedSchemaRegistryClient {
	return &CachedSchemaRegistryClient{
		registryURL: registryURL,
		auth:        auth,
		isReg:       len(registryURL) > 0,
	}
}

func (this *CachedSchemaRegistryClient) IsReg() bool {
	return this.isReg
}

func (this *CachedSchemaRegistryClient) Register(subject string, schema Schema) (int32, error) {
	var schemaIdMap map[string]int32
	var exists bool
	var id int32
	var ok bool

	schemaStr:=schema.String()
	tempSchemaIDMap, exists := this.schemaCache.Load(subject)
	if exists {
		if schemaIdMap, ok = tempSchemaIDMap.(map[string]int32); ok {
			if id, exists = schemaIdMap[schemaStr]; exists {
				return id, nil
			}
		} else {
			this.schemaCache.Delete(subject)
			schemaIdMap=make(map[string]int32)
		}
	} else {
		schemaIdMap = make(map[string]int32)
	}

	request, err := this.newDefaultRequest("POST",
		fmt.Sprintf(REGISTER_NEW_SCHEMA, subject),
		strings.NewReader(fmt.Sprintf("{\"schema\": %s}", strconv.Quote(schema.String()))))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, err
	}

	if this.isOK(response) {
		decodedResponse := &RegisterSchemaResponse{}
		if err := this.handleSuccess(response, decodedResponse); err != nil {
			return 0, err
		}

		schemaIdMap[schemaStr] = decodedResponse.Id
		this.schemaCache.Store(subject, schemaIdMap)
		this.idCache.Store(decodedResponse.Id, schema)

		return decodedResponse.Id, err
	} else {
		return 0, this.handleError(response)
	}
}

func (this *CachedSchemaRegistryClient) GetByID(id int32) (Schema, error) {
	var schema Schema
	var exists bool

	tempSchema, exists := this.idCache.Load(id)
	if exists {
		if schema, ok := tempSchema.(Schema); ok {
			return schema, nil
		}
	}

	request, err := this.newDefaultRequest("GET", fmt.Sprintf(GET_SCHEMA_BY_ID, id), nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	if this.isOK(response) {
		decodedResponse := &GetSchemaResponse{}
		if err := this.handleSuccess(response, decodedResponse); err != nil {
			return nil, err
		}
		schema, err = ParseSchema(decodedResponse.Schema)
		this.idCache.Store(id, schema)

		// this.lock.Lock()
		// this.idCache[id] = schema
		// this.lock.Unlock()
		return schema, err
	} else {
		return nil, this.handleError(response)
	}
}

func (this *CachedSchemaRegistryClient) GetLatestSchemaMetadata(subject string) (*SchemaMetadata, error) {
	request, err := this.newDefaultRequest("GET", fmt.Sprintf(GET_SPECIFIC_SUBJECT_VERSION, subject, "latest"), nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	if this.isOK(response) {
		decodedResponse := &GetSubjectVersionResponse{}
		if err := this.handleSuccess(response, decodedResponse); err != nil {
			return nil, err
		}

		return &SchemaMetadata{decodedResponse.Id, decodedResponse.Version, decodedResponse.Schema}, err
	} else {
		return nil, this.handleError(response)
	}
}

func (this *CachedSchemaRegistryClient) GetVersion(subject string, schema Schema) (int32, error) {
	var schemaVersionMap map[string]int32
	var exists bool
	var ok bool

	var version int32
	schemaStr:=schema.String()
	tempSchemaVMap, exists := this.versionCache.Load(subject)
	if exists {
		schemaVersionMap, ok = tempSchemaVMap.(map[string]int32)
		if ok {
			if version, exists = schemaVersionMap[schemaStr]; exists {
				return version, nil
			}
		} else {
			schemaVersionMap =make(map[string]int32)
			this.versionCache.Delete(subject)
		}
	} else {
		schemaVersionMap = make(map[string]int32)
	}
	decodedResponse, err := this.checkIfRegistered(subject, schema)
	if err != nil {
		return 0, err
	}
	schemaVersionMap[schemaStr] = decodedResponse.Version
	this.versionCache.Store(subject, schemaVersionMap)
	return decodedResponse.Version, nil

}

// GetIDBySchema 通过subject，schema获取schemaID
func (this *CachedSchemaRegistryClient) GetIDBySchema(subject string, schema Schema) (int32, error) {
	var schemaIDMap map[string]int32
	var exists bool
	var id int32
	var ok bool

	schemaStr:=schema.String()
	// 在缓存中查找
	tempSchemaIDMap, exists := this.schemaCache.Load(subject)
	if exists {
		schemaIDMap, ok = tempSchemaIDMap.(map[string]int32)
		if ok {
			if id, exists = schemaIDMap[schemaStr]; exists {
				return id, nil
			}
		} else {
			schemaIDMap = make(map[string]int32)
			this.schemaCache.Delete(subject)
		}
	} else {
		schemaIDMap = make(map[string]int32)
	}
	// 向注册中心请求
	decodedResponse, err := this.checkIfRegistered(subject, schema)
	if err != nil {
		return 0, err
	}
	schemaIDMap[schemaStr] = decodedResponse.Id
	this.schemaCache.Store(subject, schemaIDMap)
	return decodedResponse.Id, nil
}
func (this *CachedSchemaRegistryClient) checkIfRegistered(subject string, schema Schema) (*GetSubjectVersionResponse, error) {

	request, err := this.newDefaultRequest("POST",
		fmt.Sprintf(CHECK_IS_REGISTERED, subject),
		strings.NewReader(fmt.Sprintf("{\"schema\": %s}", strconv.Quote(schema.String()))))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	if this.isOK(response) {
		decodedResponse := &GetSubjectVersionResponse{}
		if err := this.handleSuccess(response, decodedResponse); err != nil {
			return nil, err
		}
		return decodedResponse, nil
	}

	handleErr:=this.handleError(response)

	// schema not found 发sentry
	raven.CapturePanic(func() {
		if errMsg,ok:=handleErr.(*ErrorMessage);ok{
			if errMsg.Error_code==SCHEMA_NOT_FOUND{
				panic("Schema not found")
			}
		}
	},map[string]string{"subject":subject})

	return nil, handleErr
}

func (this *CachedSchemaRegistryClient) newDefaultRequest(method string, uri string, reader io.Reader) (*http.Request, error) {
	url := fmt.Sprintf("%s%s", this.registryURL, uri)
	request, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", SCHEMA_REGISTRY_V1_JSON)
	request.Header.Set("Content-Type", SCHEMA_REGISTRY_V1_JSON)
	if this.auth != nil {
		request.Header.Set("X-Api-User", this.auth.User)
		request.Header.Set("X-Api-Key", this.auth.Key)
	}
	return request, nil
}

func (this *CachedSchemaRegistryClient) isOK(response *http.Response) bool {
	return response.StatusCode >= 200 && response.StatusCode < 300
}

func (this *CachedSchemaRegistryClient) handleSuccess(response *http.Response, model interface{}) error {
	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(responseBytes, model)
}

func (this *CachedSchemaRegistryClient) handleError(response *http.Response) error {
	registryError := &ErrorMessage{}
	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(responseBytes, registryError)
	if err != nil {
		return err
	}
	return registryError
}
