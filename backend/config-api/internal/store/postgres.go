package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"
)

// ConfigEntry represents a configuration entry
type ConfigEntry struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	Scope     string          `json:"scope"`
	UpdatedBy string          `json:"updated_by"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// PostgresStore handles database operations for configuration
type PostgresStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(host, port, user, password, dbname string, logger *slog.Logger) (*PostgresStore, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &PostgresStore{
		db:     db,
		logger: logger,
	}, nil
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// GetAllConfigs retrieves all configuration entries
func (s *PostgresStore) GetAllConfigs() ([]ConfigEntry, error) {
	query := `
		SELECT key, value, scope, updated_by, updated_at 
		FROM app_config 
		ORDER BY key
	`
	
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query configs: %w", err)
	}
	defer rows.Close()

	var configs []ConfigEntry
	for rows.Next() {
		var config ConfigEntry
		var valueStr string
		
		err := rows.Scan(&config.Key, &valueStr, &config.Scope, &config.UpdatedBy, &config.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan config: %w", err)
		}
		
		config.Value = json.RawMessage(valueStr)
		configs = append(configs, config)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return configs, nil
}

// GetConfig retrieves a specific configuration entry by key
func (s *PostgresStore) GetConfig(key string) (*ConfigEntry, error) {
	query := `
		SELECT key, value, scope, updated_by, updated_at 
		FROM app_config 
		WHERE key = $1
	`
	
	var config ConfigEntry
	var valueStr string
	
	err := s.db.QueryRow(query, key).Scan(&config.Key, &valueStr, &config.Scope, &config.UpdatedBy, &config.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query config: %w", err)
	}
	
	config.Value = json.RawMessage(valueStr)
	return &config, nil
}

// SetConfig creates or updates a configuration entry
func (s *PostgresStore) SetConfig(key string, value json.RawMessage, scope, updatedBy string) (*ConfigEntry, error) {
	query := `
		INSERT INTO app_config (key, value, scope, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			scope = EXCLUDED.scope,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
		RETURNING key, value, scope, updated_by, updated_at
	`
	
	var config ConfigEntry
	var valueStr string
	
	err := s.db.QueryRow(query, key, string(value), scope, updatedBy).Scan(
		&config.Key, &valueStr, &config.Scope, &config.UpdatedBy, &config.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to set config: %w", err)
	}
	
	config.Value = json.RawMessage(valueStr)
	return &config, nil
}

// DeleteConfig deletes a configuration entry
func (s *PostgresStore) DeleteConfig(key string) error {
	query := `DELETE FROM app_config WHERE key = $1`
	
	result, err := s.db.Exec(query, key)
	if err != nil {
		return fmt.Errorf("failed to delete config: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("config key not found: %s", key)
	}
	
	return nil
}

// Health checks if the database is accessible
func (s *PostgresStore) Health() error {
	return s.db.Ping()
}
