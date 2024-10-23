package converter

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// VideoConverter handles video conversion tasks
type VideoConverter struct {
	db *sql.DB
}

// VideoTask represents a video conversion task
type VideoTask struct {
	VideoID int    `json:"video_id"`
	Path    string `json:"path"`
}

// NewVideoConverter creates a new instance of VideoConverter
func NewVideoConverter(db *sql.DB) *VideoConverter {
	return &VideoConverter{
		db: db,
	}
}

// HandleMessage processes a video conversion message
func (vc *VideoConverter) HandleMessage(msg []byte) {
	var task VideoTask

	if err := json.Unmarshal(msg, &task); err != nil {
		vc.logError(task, "Failed to deserialize message", err)
		return
	}

	// Check if the video has already been processed
	if IsProcessed(vc.db, task.VideoID) {
		slog.Warn("Video already processed", slog.Int("video_id", task.VideoID))
		return
	}

	// Process the video
	err := vc.processVideo(&task)
	if err != nil {
		vc.logError(task, "Error during video conversion", err)
		return
	}
	slog.Info("Video conversion processed", slog.Int("video_id", task.VideoID))

	// Mark as processed
	err = MarkProcessed(vc.db, task.VideoID)
	if err != nil {
		vc.logError(task, "Failed to mark video as processed", err)
	}
	slog.Info("Video marked as processed", slog.Int("video_id", task.VideoID))
}

// processVideo handles video processing (merging chunks and converting)
func (vc *VideoConverter) processVideo(task *VideoTask) error {
	mergedFile := filepath.Join(task.Path, "merged.mp4")
	mpegDashPath := filepath.Join(task.Path, "mpeg-dash")

	// Merge chunks
	slog.Info("Merging chunks", slog.String("path", task.Path))
	if err := vc.mergeChunks(task.Path, mergedFile); err != nil {
		return fmt.Errorf("failed to merge chunks: %v", err)
	}

	// Create directory for MPEG-DASH output
	if err := os.MkdirAll(mpegDashPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Convert to MPEG-DASH
	ffmpegCmd := exec.Command(
		"ffmpeg", "-i", mergedFile, // Arquivo de entrada
		"-f", "dash", // Formato de saída
		filepath.Join(mpegDashPath, "output.mpd"), // Caminho para salvar o arquivo .mpd
	)

	output, err := ffmpegCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to convert to MPEG-DASH: %v, output: %s", err, string(output))
	}
	slog.Info("Converted to MPEG-DASH", slog.String("path", mpegDashPath))

	// Remove merged file after processing
	if err := os.Remove(mergedFile); err != nil {
		slog.Warn("Failed to remove merged file", slog.String("file", mergedFile), slog.String("error", err.Error()))
	}
	slog.Info("Removed merged file", slog.String("file", mergedFile))

	return nil
}

// Método para extrair o número do nome do arquivo
func (vc *VideoConverter) extractNumber(fileName string) int {
	re := regexp.MustCompile(`\d+`)
	numStr := re.FindString(filepath.Base(fileName)) // Pega o nome do arquivo, sem o caminho
	num, _ := strconv.Atoi(numStr)
	return num
}

// Método para mesclar os chunks
func (vc *VideoConverter) mergeChunks(inputDir, outputFile string) error {
	// Buscar todos os arquivos .chunk no diretório
	chunks, err := filepath.Glob(filepath.Join(inputDir, "*.chunk"))
	if err != nil {
		return fmt.Errorf("failed to find chunks: %v", err)
	}

	// Ordenar os chunks numericamente
	sort.Slice(chunks, func(i, j int) bool {
		return vc.extractNumber(chunks[i]) < vc.extractNumber(chunks[j])
	})

	// Criar arquivo de saída
	output, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create merged file: %v", err)
	}
	defer output.Close()

	// Ler cada chunk e escrever no arquivo final
	for _, chunk := range chunks {
		input, err := os.Open(chunk)
		if err != nil {
			return fmt.Errorf("failed to open chunk %s: %v", chunk, err)
		}

		// Copiar dados do chunk para o arquivo de saída
		_, err = output.ReadFrom(input)
		if err != nil {
			return fmt.Errorf("failed to write chunk %s to merged file: %v", chunk, err)
		}
		input.Close()
	}
	return nil
}

// logError handles logging the error in JSON format
func (vc *VideoConverter) logError(task VideoTask, message string, err error) {
	errorData := map[string]interface{}{
		"video_id": task.VideoID,
		"error":    message,
		"details":  err.Error(),
		"time":     time.Now(),
	}

	serializedError, _ := json.Marshal(errorData)
	slog.Error("Processing error", slog.String("error_details", string(serializedError)))

	RegisterError(vc.db, errorData, err)
}
