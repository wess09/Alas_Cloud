package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm/clause"
)

type telemetryBuffer struct {
	mu            sync.Mutex
	pending       map[string]models.TelemetryData
	flushInterval time.Duration
	flushSize     int
	flushSignal   chan struct{}
	stopCh        chan struct{}
	doneCh        chan struct{}
}

var telemetryWriter *telemetryBuffer

func InitTelemetryWriter() {
	if telemetryWriter != nil {
		return
	}

	telemetryWriter = &telemetryBuffer{
		pending:       make(map[string]models.TelemetryData),
		flushInterval: loadTelemetryFlushInterval(),
		flushSize:     loadTelemetryFlushSize(),
		flushSignal:   make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	go telemetryWriter.run()
	log.Printf("[TELEMETRY] buffered writer started interval=%s flush_size=%d", telemetryWriter.flushInterval, telemetryWriter.flushSize)
}

func ShutdownTelemetryWriter(ctx context.Context) error {
	if telemetryWriter == nil {
		return nil
	}

	close(telemetryWriter.stopCh)
	select {
	case <-telemetryWriter.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func EnqueueTelemetryWrite(data models.TelemetryData) {
	if telemetryWriter == nil {
		writeTelemetryBatch([]models.TelemetryData{data})
		return
	}

	telemetryWriter.enqueue(data)
}

func (b *telemetryBuffer) run() {
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	defer close(b.doneCh)

	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.flushSignal:
			b.flush()
		case <-b.stopCh:
			b.flush()
			return
		}
	}
}

func (b *telemetryBuffer) enqueue(data models.TelemetryData) {
	b.mu.Lock()
	key := telemetryBufferKey(data)
	existing, exists := b.pending[key]
	if exists && !existing.CreatedAt.IsZero() {
		data.CreatedAt = existing.CreatedAt
	}
	b.pending[key] = data
	shouldFlush := len(b.pending) >= b.flushSize
	b.mu.Unlock()

	if shouldFlush {
		select {
		case b.flushSignal <- struct{}{}:
		default:
		}
	}
}

func (b *telemetryBuffer) flush() {
	batch := b.drain()
	if len(batch) == 0 {
		return
	}

	if err := writeTelemetryBatch(batch); err != nil {
		log.Printf("[TELEMETRY] buffered flush failed, requeueing %d rows: %v", len(batch), err)
		b.requeue(batch)
		return
	}

	log.Printf("[TELEMETRY] flushed %d buffered rows", len(batch))
}

func (b *telemetryBuffer) drain() []models.TelemetryData {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.pending) == 0 {
		return nil
	}

	batch := make([]models.TelemetryData, 0, len(b.pending))
	for _, item := range b.pending {
		batch = append(batch, item)
	}
	b.pending = make(map[string]models.TelemetryData)
	return batch
}

func (b *telemetryBuffer) requeue(batch []models.TelemetryData) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, item := range batch {
		key := telemetryBufferKey(item)
		existing, exists := b.pending[key]
		if exists && existing.UpdatedAt.After(item.UpdatedAt) {
			continue
		}
		if exists && !existing.CreatedAt.IsZero() {
			item.CreatedAt = existing.CreatedAt
		}
		b.pending[key] = item
	}
}

func writeTelemetryBatch(batch []models.TelemetryData) error {
	if len(batch) == 0 {
		return nil
	}

	err := database.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "device_id"},
			{Name: "instance_id"},
			{Name: "month"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"battle_count",
			"battle_rounds",
			"sortie_cost",
			"akashi_encounters",
			"akashi_probability",
			"average_stamina",
			"net_stamina_gain",
			"ip_address",
			"updated_at",
		}),
	}).CreateInBatches(batch, 200).Error
	if err != nil {
		return err
	}

	invalidateTelemetryCache()
	RequestStatsRefresh()
	return nil
}

func telemetryBufferKey(data models.TelemetryData) string {
	return data.DeviceID + "|" + data.InstanceID + "|" + data.Month
}

func loadTelemetryFlushInterval() time.Duration {
	raw := os.Getenv("TELEMETRY_FLUSH_INTERVAL_SECONDS")
	if raw == "" {
		return 10 * time.Second
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 10 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func loadTelemetryFlushSize() int {
	raw := os.Getenv("TELEMETRY_FLUSH_BATCH_SIZE")
	if raw == "" {
		return 500
	}

	size, err := strconv.Atoi(raw)
	if err != nil || size <= 0 {
		return 500
	}
	return size
}
