package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/go-jet/jet/v2/generator/metadata"
	"github.com/go-jet/jet/v2/generator/postgres"
	"github.com/go-jet/jet/v2/generator/template"
	postgres2 "github.com/go-jet/jet/v2/postgres"
	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

type DBConfig struct {
	Name      string
	User      string
	Password  string
	Port      int
	DBName    string
	Schema    string
	SSLMode   string
	OutputDir string
}

type AdminDBConfig struct {
	User     string
	Password string
	Port     int
	SSLMode  string
}

func useArray(f *template.TableModelField, _ metadata.Table, c metadata.Column) {
	if c.DataType.Name == "text[]" {
		f.Type = template.NewType(pq.StringArray{})
	}
}

func generateModels(config DBConfig) error {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	dbConnection := postgres.DBConnection{
		User:       config.User,
		Password:   config.Password,
		Port:       config.Port,
		DBName:     config.DBName,
		SchemaName: config.Schema,
		SslMode:    config.SSLMode,
	}

	err := postgres.Generate(
		config.OutputDir,
		dbConnection,
		template.Default(postgres2.Dialect).
			UseSchema(
				func(schemaMetaData metadata.Schema) template.Schema {
					return template.DefaultSchema(schemaMetaData).
						UseModel(
							template.DefaultModel().
								UseTable(
									func(t metadata.Table) template.TableModel {
										return template.DefaultTableModel(t).
											UseField(
												func(c metadata.Column) template.TableModelField {
													field := template.DefaultTableModelField(c)
													useArray(&field, t, c)
													return field
												},
											)
									},
								),
						)
				},
			),
	)
	if err != nil {
		return fmt.Errorf("error generating models for DB '%s': %w", config.Name, err)
	}
	log.Printf("Successfully generated models for DB '%s' at '%s'\n", config.Name, config.OutputDir)
	return nil
}

func ensureDatabaseExists(adminConfig AdminDBConfig, dbName string) error {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	connStr := fmt.Sprintf(
		"host=localhost port=%d user=%s password=%s dbname=postgres sslmode=%s",
		adminConfig.Port, adminConfig.User, adminConfig.Password, adminConfig.SSLMode,
	)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to admin database: %w", err)
	}
	defer db.Close()

	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)"
	err = db.QueryRow(query, dbName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database '%s' exists: %w", dbName, err)
	}

	if exists {
		log.Printf("Database '%s' already exists.\n", dbName)
	} else {
		createQuery := fmt.Sprintf("CREATE DATABASE %s", pq.QuoteIdentifier(dbName))
		_, err = db.Exec(createQuery)
		if err != nil {
			return fmt.Errorf("failed to create database '%s': %w", dbName, err)
		}
		log.Printf("Database '%s' created successfully.\n", dbName)
	}

	return nil
}

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	adminConfig := AdminDBConfig{
		User:     "postgres",
		Password: getEnv("ANCHOR_DB_PASSWORD", ""),
		//nolint:mnd //Default port for PostgreSQL
		Port:    getEnvAsInt("ANCHOR_DB_PORT", 5432),
		SSLMode: "disable",
	}

	if adminConfig.Password == "" {
		log.Fatal().Msgf("ADMIN_DB_PASSWORD environment variable is required for admin connection")
	}

	databases := []DBConfig{
		{
			Name:     "anchor",
			User:     "postgres",
			Password: getEnv("ANCHOR_DB_PASSWORD", ""),
			//nolint:mnd //Default port for PostgreSQL
			Port:      getEnvAsInt("ANCHOR_DB_PORT", 5432),
			DBName:    getEnv("CORE_DB_NAME", "anchor"),
			Schema:    "public",
			SSLMode:   "disable",
			OutputDir: "./internal/db/gen",
		},
	}

	// Validate and ensure databases exist
	for _, dbConfig := range databases {
		if dbConfig.Password == "" {
			log.Fatal().Msgf("%s_DB_PASSWORD environment variable is required", dbConfig.Name)
		}
		if dbConfig.DBName == "" {
			log.Fatal().Msgf("%s_DB_NAME environment variable is required", dbConfig.Name)
		}

		// Ensure the database exists
		if err := ensureDatabaseExists(adminConfig, dbConfig.DBName); err != nil {
			log.Fatal().Msgf("Failed to ensure database '%s': %v", dbConfig.DBName, err)
		}

		// Generate models for the database
		if err := generateModels(dbConfig); err != nil {
			log.Fatal().Err(err)
		}
	}
}

// getEnv reads an environment variable or returns a default value.
func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultVal
}

// getEnvAsInt reads an environment variable as integer or returns a default value.
func getEnvAsInt(name string, defaultVal int) int {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if valueStr, exists := os.LookupEnv(name); exists && valueStr != "" {
		value, err := strconv.Atoi(valueStr)
		if err != nil {
			log.Fatal().Msgf("Invalid value for %s: %v", name, err)
		}
		return value
	}
	return defaultVal
}
