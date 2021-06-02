package mongodb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"path/filepath"
	"strconv"
)


type ClientConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	DB		 string
	Ssl      bool
	InsecureSkipVerify bool
	ReplicaSet string
	Ca       string
	Cert     string
	Key      string
	CertPath string
	RetryWrites int
}
type DbUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type Role struct {
	Role string `json:"role"`
	Db   string `json:"db"`
}
func (role Role) String() string {
	return fmt.Sprintf("{ role : %s , db : %s }", role.Role, role.Db)
}
type PrivilegeDto struct {
	Db         string `json:"db"`
	Collection string `json:"collection"`
	Actions  []string `json:"actions"`
}

type Privilege struct {
	Resource Resource `json:"resource"`
	Actions  []string `json:"actions"`
}
func addArgs(arguments string,newArg string) string {
	if arguments != "" {
		return arguments+"&"+newArg
	} else {
		return "/?"+newArg
	}

}

func (c *ClientConfig) MongoClient() (*mongo.Client, error) {

	if c.Cert != "" || c.Key != "" {
		if c.Cert == "" || c.Key == "" {
			return nil, fmt.Errorf("cert_material, and key_material must be specified")
		}

		if c.CertPath != "" {
			return nil, fmt.Errorf("cert_path must not be specified")
		}

		mongoClient, err := buildHTTPClientFromBytes([]byte(c.Ca), []byte(c.Cert), []byte(c.Key), c)
		if err != nil {
			return nil, err
		}
		return mongoClient,err
	}
	if c.CertPath != "" {
		// If there is cert information, load it and use it.
		ca := filepath.Join(c.CertPath, "ca.pem")
		cert := filepath.Join(c.CertPath, "cert.pem")
		key := filepath.Join(c.CertPath, "key.pem")
		mongoClient , err := buildHttpClientFromCertPath([]byte(ca), []byte(cert) , []byte(key) , c)
		if err != nil {
			return nil, err
		}
		return mongoClient,err
	}
	var arguments = ""
	if c.RetryWrites != -1 {
		arguments = addArgs(arguments,"retrywrites="+strconv.FormatBool(c.RetryWrites == 1))
	}
	if c.Ssl {
		arguments = addArgs(arguments,"ssl=true")
	}
	if c.ReplicaSet != "" {
		arguments = addArgs(arguments,"replicaSet="+c.ReplicaSet)
	}
	var uri = "mongodb://" + c.Host + ":" + c.Port + arguments

	if c.Ca != "" {
		tlsConfig, err := getTLSConfigWithAllServerCertificates([]byte(c.Ca))
		if err != nil {
			return nil, err
		}

		mongoClient, err := mongo.NewClient(options.Client().ApplyURI(uri).SetAuth(options.Credential{
			AuthSource: c.DB, Username: c.Username, Password: c.Password,
		}).SetTLSConfig(tlsConfig))

		return mongoClient, err
	}

	client, err := mongo.NewClient(options.Client().ApplyURI(uri).SetAuth(options.Credential{
		AuthSource: c.DB, Username: c.Username, Password: c.Password,
	}))
	return client, err
}

func getTLSConfigWithAllServerCertificates(ca []byte) (*tls.Config, error) {
	/* As of version 1.2.1, the MongoDB Go Driver will only use the first CA server certificate found in sslcertificateauthorityfile.
	   The code below addresses this limitation by manually appending all server certificates found in sslcertificateauthorityfile
	   to a custom TLS configuration used during client creation. */

	tlsConfig := new(tls.Config)

	tlsConfig.RootCAs = x509.NewCertPool()
	ok := tlsConfig.RootCAs.AppendCertsFromPEM(ca)

	if !ok {
		return tlsConfig, errors.New("Failed parsing pem file")
	}

	return tlsConfig, nil
}

func buildHttpClientFromCertPath(ca , cert , key []byte, config *ClientConfig) (*mongo.Client, error) {
	tlsConfig := &tls.Config{}
	if cert != nil && key != nil {
		tlsCert, err := tls.X509KeyPair(cert, key)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	} else {
		tlsConfig.InsecureSkipVerify = true
	}
	if ca == nil || len(ca) == 0 {
		tlsConfig.InsecureSkipVerify = true
	} else {
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(ca) {
			return nil, errors.New("Could not add RootCA pem")
		}
		tlsConfig.RootCAs = caPool
	}
	var arguments = ""
	if config.RetryWrites != -1 {
		arguments = addArgs(arguments,"retrywrites="+strconv.FormatBool(config.RetryWrites == 1))
	}
	if config.Ssl {
		arguments = addArgs(arguments,"ssl=true")
	}
	if config.ReplicaSet != "" {
		arguments = addArgs(arguments,"replicaSet="+config.ReplicaSet)
	}
	var uri = "mongodb://" + config.Host + ":" + config.Port + arguments

	client, err := mongo.NewClient(options.Client().ApplyURI(uri).SetAuth(options.Credential{
		AuthSource: config.DB, Username: config.Username , Password: config.Password,
	}).SetTLSConfig(tlsConfig))

	return client , err

}
func buildHTTPClientFromBytes(caPEMCert, certPEMBlock, keyPEMBlock []byte, config *ClientConfig) (*mongo.Client, error) {
	tlsConfig := &tls.Config{}
	if certPEMBlock != nil && keyPEMBlock != nil {
		tlsCert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}

	if caPEMCert == nil || len(caPEMCert) == 0 {
		tlsConfig.InsecureSkipVerify = true
	} else {
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPEMCert) {
			return nil, errors.New("Could not add RootCA pem")
		}
		tlsConfig.RootCAs = caPool
	}
	if config.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}
	var arguments = ""
	if config.RetryWrites != -1 {
		arguments = addArgs(arguments,"retrywrites="+strconv.FormatBool(config.RetryWrites == 1))
	}
	if config.Ssl {
		arguments = addArgs(arguments,"ssl=true")
	}
	if config.ReplicaSet != "" {
		arguments = addArgs(arguments,"replicaSet="+config.ReplicaSet)
	}
	var uri = "mongodb://" + config.Host + ":" + config.Port + arguments

	client, err := mongo.NewClient(options.Client().ApplyURI(uri).SetAuth(options.Credential{
			AuthSource: config.DB, Username: config.Username , Password: config.Password,
		}).SetTLSConfig(tlsConfig))

	return client , err
}

func (privilege Privilege) String() string {
	return fmt.Sprintf("{ resource : %s , actions : %s }", privilege.Resource, privilege.Actions)
}

type Resource struct {
	Db         string `json:"db"`
	Collection string `json:"collection"`
}

func (resource Resource) String() string {
	return fmt.Sprintf(" { db : %s , collection : %s }", resource.Db, resource.Collection)
}


func createUser(client *mongo.Client, user DbUser, roles []Role, database string) error {
	var result *mongo.SingleResult
	if len(roles) != 0  {
		result = client.Database(database).RunCommand(context.Background(), bson.D{{Key: "createUser", Value: user.Name},
			{Key: "pwd", Value: user.Password}, {Key: "roles", Value: roles}})
	} else{
		result = client.Database(database).RunCommand(context.Background(), bson.D{{Key: "createUser", Value: user.Name},
			{Key: "pwd", Value: user.Password}, {Key: "roles", Value: []bson.M{}}})
	}

	if result.Err() != nil {
		return result.Err()
	}
	return nil
}

func createRole(client *mongo.Client, role string, roles []Role, privilege []PrivilegeDto, database string) error {
	var privileges []Privilege
	var result *mongo.SingleResult
	for _ , element := range privilege {
		var prv Privilege
		prv.Resource = Resource{
			Db:         element.Db,
			Collection: element.Collection,
		}
		prv.Actions = element.Actions
		privileges = append(privileges,prv)
	}
	if len(roles) != 0 && len(privileges) != 0 {
		result = client.Database(database).RunCommand(context.Background(), bson.D{{Key: "createRole", Value: role},
			{Key: "privileges", Value: privileges}, {Key: "roles", Value: roles}})
	}else if len(roles) == 0 && len(privileges) != 0 {
		result = client.Database(database).RunCommand(context.Background(), bson.D{{Key: "createRole", Value: role},
			{Key: "privileges", Value: privileges}, {Key: "roles", Value: []bson.M{}}})
	}else if len(roles) != 0 && len(privileges) == 0 {
		result = client.Database(database).RunCommand(context.Background(), bson.D{{Key: "createRole", Value: role},
			{Key: "privileges", Value: []bson.M{}}, {Key: "roles", Value: roles}})
	}else{
		result = client.Database(database).RunCommand(context.Background(), bson.D{{Key: "createRole", Value: role},
			{Key: "privileges", Value: []bson.M{}}, {Key: "roles", Value: []bson.M{}}})
	}

	if result.Err() != nil {
		return result.Err()
	}
	return nil
}
