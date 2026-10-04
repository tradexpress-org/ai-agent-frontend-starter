package chat

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"encore.dev/beta/errs"
	"github.com/google/uuid"
)

const maxDatasetNameLength = 255

var errDatasetNameExists = errors.New("dataset name already exists")

// DatasetSummary describes a stored dataset without its records.
type DatasetSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RecordCount int    `json:"record_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// CreateDatasetResult is returned after importing a dataset.
type CreateDatasetResult struct {
	DatasetSummary
}

// DatasetResponse contains a stored dataset and its records.
type DatasetResponse struct {
	DatasetSummary
	Records []map[string]any `json:"records"`
}

// ListDatasetsResponse contains all stored dataset summaries.
type ListDatasetsResponse struct {
	Datasets []DatasetSummary `json:"datasets"`
}

// RenameDatasetRequest is the request to change a dataset's name.
type RenameDatasetRequest struct {
	Name string `json:"name"`
}

// ImportDataset imports a JSON or CSV file. JSON files must contain an array
// of objects; CSV headers become record fields.
//
//encore:api public raw method=POST path=/datasets
func ImportDataset(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		if req.MultipartForm != nil {
			_ = req.MultipartForm.RemoveAll()
		}
		writeDatasetError(w, http.StatusBadRequest, "expected a multipart form with a name and file")
		return
	}
	if req.MultipartForm != nil {
		defer req.MultipartForm.RemoveAll()
	}

	name, err := normalizeDatasetName(req.FormValue("name"))
	if err != nil {
		writeDatasetError(w, http.StatusBadRequest, err.Error())
		return
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		writeDatasetError(w, http.StatusBadRequest, "a dataset file is required")
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		writeDatasetError(w, http.StatusBadRequest, "could not read dataset file")
		return
	}
	records, err := parseDatasetFile(header.Filename, header.Header.Get("Content-Type"), content)
	if err != nil {
		writeDatasetError(w, http.StatusBadRequest, err.Error())
		return
	}

	dataset, err := insertDataset(req.Context(), name, records)
	if errors.Is(err, errDatasetNameExists) {
		writeDatasetError(w, http.StatusConflict, "a dataset with that name already exists")
		return
	}
	if err != nil {
		log.Printf("import dataset: %v", err)
		writeDatasetError(w, http.StatusInternalServerError, "could not save dataset")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(dataset); err != nil {
		log.Printf("write dataset import response: %v", err)
	}
}

// ListDatasets lists the stored datasets.
//
//encore:api public method=GET path=/datasets
func ListDatasets(ctx context.Context) (*ListDatasetsResponse, error) {
	rows, err := db.Query(ctx, `
		SELECT id, name, jsonb_array_length(records), created_at, updated_at
		FROM datasets
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var datasets []DatasetSummary
	for rows.Next() {
		var dataset DatasetSummary
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&dataset.ID, &dataset.Name, &dataset.RecordCount, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		dataset.CreatedAt = createdAt.Format(time.RFC3339)
		dataset.UpdatedAt = updatedAt.Format(time.RFC3339)
		datasets = append(datasets, dataset)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if datasets == nil {
		datasets = []DatasetSummary{}
	}
	return &ListDatasetsResponse{Datasets: datasets}, nil
}

// GetDataset returns a dataset and its records.
//
//encore:api public method=GET path=/datasets/:datasetID
func GetDataset(ctx context.Context, datasetID string) (*DatasetResponse, error) {
	if _, err := uuid.Parse(datasetID); err != nil {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "invalid dataset ID"}
	}

	rows, err := db.Query(ctx, `
		SELECT id, name, records, jsonb_array_length(records), created_at, updated_at
		FROM datasets
		WHERE id = $1
	`, datasetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, &errs.Error{Code: errs.NotFound, Message: "dataset not found"}
	}

	var dataset DatasetResponse
	var recordsJSON []byte
	var createdAt, updatedAt time.Time
	if err := rows.Scan(
		&dataset.ID,
		&dataset.Name,
		&recordsJSON,
		&dataset.RecordCount,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(recordsJSON, &dataset.Records); err != nil {
		return nil, fmt.Errorf("decode stored dataset records: %w", err)
	}
	dataset.CreatedAt = createdAt.Format(time.RFC3339)
	dataset.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &dataset, nil
}

// RenameDataset changes the name of a stored dataset.
//
//encore:api public method=PATCH path=/datasets/:datasetID
func RenameDataset(ctx context.Context, datasetID string, req *RenameDatasetRequest) (*DatasetSummary, error) {
	if _, err := uuid.Parse(datasetID); err != nil {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "invalid dataset ID"}
	}
	name, err := normalizeDatasetName(req.Name)
	if err != nil {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: err.Error()}
	}

	rows, err := db.Query(ctx, `
		UPDATE datasets AS target
		SET name = $2, updated_at = NOW()
		WHERE target.id = $1
		  AND NOT EXISTS (
			SELECT 1 FROM datasets AS existing
			WHERE existing.name = $2 AND existing.id <> target.id
		  )
		RETURNING id, name, jsonb_array_length(records), created_at, updated_at
	`, datasetID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var dataset DatasetSummary
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&dataset.ID, &dataset.Name, &dataset.RecordCount, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		dataset.CreatedAt = createdAt.Format(time.RFC3339)
		dataset.UpdatedAt = updatedAt.Format(time.RFC3339)
		return &dataset, nil
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var exists bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM datasets WHERE id = $1)`, datasetID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, &errs.Error{Code: errs.NotFound, Message: "dataset not found"}
	}
	return nil, &errs.Error{Code: errs.AlreadyExists, Message: "a dataset with that name already exists"}
}

func insertDataset(ctx context.Context, name string, records []map[string]any) (*CreateDatasetResult, error) {
	recordsJSON, err := json.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("encode dataset records: %w", err)
	}

	rows, err := db.Query(ctx, `
		INSERT INTO datasets (id, name, records)
		VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (name) DO NOTHING
		RETURNING id, name, jsonb_array_length(records), created_at, updated_at
	`, uuid.New().String(), name, string(recordsJSON))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, errDatasetNameExists
	}

	var dataset CreateDatasetResult
	var createdAt, updatedAt time.Time
	if err := rows.Scan(&dataset.ID, &dataset.Name, &dataset.RecordCount, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	dataset.CreatedAt = createdAt.Format(time.RFC3339)
	dataset.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &dataset, nil
}

func normalizeDatasetName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("dataset name is required")
	}
	if utf8.RuneCountInString(name) > maxDatasetNameLength {
		return "", fmt.Errorf("dataset name must be %d characters or fewer", maxDatasetNameLength)
	}
	return name, nil
}

func parseDatasetFile(filename, contentType string, content []byte) ([]map[string]any, error) {
	extension := strings.ToLower(filepath.Ext(filename))
	mediaType, _, _ := mime.ParseMediaType(contentType)

	switch {
	case extension == ".json" || mediaType == "application/json":
		return parseJSONRecords(content)
	case extension == ".csv" || mediaType == "text/csv":
		return parseCSVRecords(content)
	default:
		return nil, errors.New("dataset file must be JSON or CSV")
	}
}

func parseJSONRecords(content []byte) ([]map[string]any, error) {
	var records []map[string]any
	if err := json.Unmarshal(content, &records); err != nil {
		return nil, fmt.Errorf("invalid JSON dataset: expected an array of objects: %w", err)
	}
	if records == nil {
		return nil, errors.New("invalid JSON dataset: expected an array of objects")
	}
	for i, record := range records {
		if record == nil {
			return nil, fmt.Errorf("invalid JSON dataset: record %d must be an object", i+1)
		}
	}
	return records, nil
}

func parseCSVRecords(content []byte) ([]map[string]any, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV dataset: %w", err)
	}
	if len(rows) == 0 {
		return nil, errors.New("invalid CSV dataset: a header row is required")
	}

	headers := rows[0]
	seen := make(map[string]struct{}, len(headers))
	for i, header := range headers {
		header = strings.TrimSpace(strings.TrimPrefix(header, "\ufeff"))
		if header == "" {
			return nil, fmt.Errorf("invalid CSV dataset: column %d has an empty header", i+1)
		}
		if _, exists := seen[header]; exists {
			return nil, fmt.Errorf("invalid CSV dataset: duplicate header %q", header)
		}
		seen[header] = struct{}{}
		headers[i] = header
	}

	records := make([]map[string]any, 0, len(rows)-1)
	for rowIndex, row := range rows[1:] {
		if len(row) != len(headers) {
			return nil, fmt.Errorf("invalid CSV dataset: row %d has %d fields; expected %d", rowIndex+2, len(row), len(headers))
		}
		record := make(map[string]any, len(headers))
		for i, value := range row {
			record[headers[i]] = value
		}
		records = append(records, record)
	}
	return records, nil
}

func writeDatasetError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: message}); err != nil {
		log.Printf("write dataset error response: %v", err)
	}
}
