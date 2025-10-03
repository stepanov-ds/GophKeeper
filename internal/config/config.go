package config

import (
	"flag"
	"log"
	"os"
	"time"
)

var (
	EndpointServer      = flag.String("a", "0.0.0.0:8085", "endpoint")
	DatabaseDSN         = flag.String("d", "", "database_DSN")
	RegistrationEnabled = flag.Bool("e", true, "enables registration page")
	CleanupTime         = flag.Duration("t", time.Minute, "cache cleanup time")
	jwtKeyString        = flag.String("j", "default", "JWT key string")
	JWTKey              []byte
)

func ConfigServer() {
	address, found := os.LookupEnv("ADDRESS")
	if found {
		EndpointServer = &address
	}
	dsn, found := os.LookupEnv("DATABASE_DSN_GOPHKKEEPER")
	if found {
		DatabaseDSN = &dsn
	}
	flag.Parse()
	JWTKey = []byte(*jwtKeyString)

	log.Println("Server configuration:",
		"\nEndpointServer:", *EndpointServer,
		"\nDatabaseDSN:", *DatabaseDSN,
		"\nRegistration Page enabled:", *RegistrationEnabled)
}
