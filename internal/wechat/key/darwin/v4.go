package darwin

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"runtime"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/decrypt"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/decrypt/common"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/key/darwin/glance"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/model"
)

const (
	MaxWorkers        = 8
	MinChunkSize      = 1 * 1024 * 1024 // 1MB
	ChunkOverlapBytes = 1024            // Greater than all offsets
	ChunkMultiplier   = 2               // Number of chunks = MaxWorkers * ChunkMultiplier
)

var V4ImgKeyPatterns = []KeyPatternInfo{
	{
		Pattern: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		Offsets: []int{-32},
	},
}

type V4Extractor struct {
	validator            *decrypt.Validator
	imgKeyPatterns       []KeyPatternInfo
	processedImgKeys     sync.Map          // Thread-safe map for processed image keys
	processedDerivedKeys sync.Map          // Thread-safe map for processed derived key candidates
	derivedKeyMu         sync.Mutex        // Protects derivedKeyMap
	derivedKeyMap        map[string]string // dbRelPath -> keyHex
}

func NewV4Extractor() *V4Extractor {
	return &V4Extractor{
		imgKeyPatterns: V4ImgKeyPatterns,
		derivedKeyMap:  make(map[string]string),
	}
}

func (e *V4Extractor) Extract(ctx context.Context, proc *model.Process) (string, map[string]string, error) {
	if proc.Status == model.StatusOffline {
		return "", nil, errors.ErrWeChatOffline
	}

	// Check if SIP is disabled, as it's required for memory reading on macOS
	if !glance.IsSIPDisabled() {
		return "", nil, errors.ErrSIPEnabled
	}

	if e.validator == nil {
		return "", nil, errors.ErrValidatorNotSet
	}

	log.Info().Uint32("pid", proc.PID).Str("account", proc.Name).Msg("开始从进程内存提取密钥")

	// scan db files for multi-db derived-key validation; ignore errors (degrades to single-db)
	if err := e.validator.ScanAllDBFiles(); err != nil {
		log.Debug().Err(err).Msg("ScanAllDBFiles encountered an error, derived key scan may be incomplete")
	}

	// Create context to control all goroutines
	searchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create channels for memory data and results
	memoryChannel := make(chan []byte, 100)
	resultChannel := make(chan string, 1)

	// Determine number of worker goroutines
	workerCount := runtime.NumCPU()
	if workerCount < 2 {
		workerCount = 2
	}
	if workerCount > MaxWorkers {
		workerCount = MaxWorkers
	}
	log.Debug().Msgf("Starting %d workers for V4 key search", workerCount)

	// Start consumer goroutines
	var workerWaitGroup sync.WaitGroup
	workerWaitGroup.Add(workerCount)
	for index := 0; index < workerCount; index++ {
		go func() {
			defer workerWaitGroup.Done()
			e.worker(searchCtx, memoryChannel, resultChannel)
		}()
	}

	// Start producer goroutine
	var producerWaitGroup sync.WaitGroup
	producerWaitGroup.Add(1)
	go func() {
		defer producerWaitGroup.Done()
		defer close(memoryChannel) // Close channel when producer is done
		err := e.findMemory(searchCtx, uint32(proc.PID), memoryChannel)
		if err != nil {
			log.Err(err).Msg("Failed to read memory")
		}
	}()

	// Wait for producer and consumers to complete
	go func() {
		producerWaitGroup.Wait()
		workerWaitGroup.Wait()
		close(resultChannel)
	}()

	// Wait for result
	var finalImgKey string

	for {
		select {
		case <-ctx.Done():
			return "", nil, ctx.Err()
		case imgKey, ok := <-resultChannel:
			if !ok {
				// channel closed: all workers done
				derivedKeyMap := e.GetDerivedKeyMap()
				if finalImgKey != "" || len(derivedKeyMap) > 0 {
					log.Info().
						Bool("img_key_found", finalImgKey != "").
						Int("derived_keys", len(derivedKeyMap)).
						Msg("密钥提取完成")
					return finalImgKey, derivedKeyMap, nil
				}
				log.Warn().Msg("内存扫描完成，未找到有效密钥")
				return "", nil, errors.ErrNoValidKey
			}

			if imgKey != "" {
				finalImgKey = imgKey
			}
		}
	}
}

// findMemory searches for memory regions using Glance
func (e *V4Extractor) findMemory(ctx context.Context, pid uint32, memoryChannel chan<- []byte) error {
	// Initialize a Glance instance to read process memory
	g := glance.NewGlance(pid)

	// Read memory data
	memory, err := g.Read()
	if err != nil {
		return err
	}

	totalSize := len(memory)
	log.Debug().Msgf("Read memory region, size: %d bytes", totalSize)

	// If memory is small enough, process it as a single chunk
	if totalSize <= MinChunkSize {
		select {
		case memoryChannel <- memory:
			log.Debug().Msg("Memory sent as a single chunk for analysis")
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	}

	chunkCount := MaxWorkers * ChunkMultiplier

	// Calculate chunk size based on fixed chunk count
	chunkSize := totalSize / chunkCount
	if chunkSize < MinChunkSize {
		// Reduce number of chunks if each would be too small
		chunkCount = totalSize / MinChunkSize
		if chunkCount == 0 {
			chunkCount = 1
		}
		chunkSize = totalSize / chunkCount
	}

	// Process memory in chunks from end to beginning
	for i := chunkCount - 1; i >= 0; i-- {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Calculate start and end positions for this chunk
			start := i * chunkSize
			end := (i + 1) * chunkSize

			// Ensure the last chunk includes all remaining memory
			if i == chunkCount-1 {
				end = totalSize
			}

			// Add overlap area to catch patterns at chunk boundaries
			if i > 0 {
				start -= ChunkOverlapBytes
				if start < 0 {
					start = 0
				}
			}

			chunk := memory[start:end]

			log.Debug().
				Int("chunk_index", i+1).
				Int("total_chunks", chunkCount).
				Int("chunk_size", len(chunk)).
				Str("start_offset", fmt.Sprintf("%X", start)).
				Str("end_offset", fmt.Sprintf("%X", end)).
				Msg("Processing memory chunk")

			select {
			case memoryChannel <- chunk:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return nil
}

// worker processes memory regions to find V4 version key
func (e *V4Extractor) worker(ctx context.Context, memoryChannel <-chan []byte, resultChannel chan<- string) {
	var imgKey string

	for {
		select {
		case <-ctx.Done():
			return
		case memory, ok := <-memoryChannel:
			if !ok {
				if imgKey != "" {
					select {
					case resultChannel <- imgKey:
					default:
					}
				}
				return
			}

			// scan for derived encKeys; collect into derivedKeyMap
			e.SearchDerivedKeys(ctx, memory)

			//if imgKey == "" {
			//	if key, ok := e.SearchImgKey(ctx, memory); ok {
			//		imgKey = key
			//		log.Debug().Msg("Image key found: " + key)
			//		select {
			//		case resultChannel <- imgKey:
			//		case <-ctx.Done():
			//			return
			//		}
			//	}
			//}
		}
	}
}

func (e *V4Extractor) SearchImgKey(ctx context.Context, memory []byte) (string, bool) {

	for _, keyPattern := range e.imgKeyPatterns {
		index := len(memory)

		for {
			select {
			case <-ctx.Done():
				return "", false
			default:
			}

			// Find pattern from end to beginning
			index = bytes.LastIndex(memory[:index], keyPattern.Pattern)
			if index == -1 {
				break // No more matches found
			}

			// align to 16 bytes
			index = bytes.LastIndexFunc(memory[:index], func(r rune) bool {
				return r != 0
			})

			if index == -1 {
				break // No more matches found
			}

			index += 1

			// Try each offset for this pattern
			for _, offset := range keyPattern.Offsets {
				// Check if we have enough space for the key (16 bytes for image key)
				keyOffset := index + offset
				if keyOffset < 0 || keyOffset+16 > len(memory) {
					continue
				}

				if bytes.Equal(memory[keyOffset:keyOffset+16], []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) {
					continue
				}

				// Extract the key data, which is at the offset position and 16 bytes long
				keyData := memory[keyOffset : keyOffset+16]
				keyHex := hex.EncodeToString(keyData)

				// Skip if we've already processed this key (thread-safe check)
				if _, loaded := e.processedImgKeys.LoadOrStore(keyHex, true); loaded {
					continue
				}

				// Validate key using image key validator
				if e.validator.ValidateImgKey(keyData) {
					log.Debug().
						Str("pattern", hex.EncodeToString(keyPattern.Pattern)).
						Int("offset", offset).
						Str("key", keyHex).
						Msg("Image key found")
					return keyHex, true
				}
			}

			index -= 1
			if index < 0 {
				break
			}
		}
	}

	return "", false
}

// SearchDerivedKeys scans memory in 32-byte strides and validates each candidate
// against all known database files, collecting matches into e.derivedKeyMap.
func (e *V4Extractor) SearchDerivedKeys(ctx context.Context, memory []byte) {
	zeroKey := make([]byte, common.KeySize)
	for i := 0; i+common.KeySize <= len(memory); i += common.KeySize {
		select {
		case <-ctx.Done():
			return
		default:
		}

		candidate := memory[i : i+common.KeySize]
		if bytes.Equal(candidate, zeroKey) {
			continue
		}

		keyHex := hex.EncodeToString(candidate)
		if _, loaded := e.processedDerivedKeys.LoadOrStore(keyHex, true); loaded {
			continue
		}

		matchingDBs := e.validator.ValidateDerivedAll(candidate)
		if len(matchingDBs) > 0 {
			e.derivedKeyMu.Lock()
			for _, dbPath := range matchingDBs {
				e.derivedKeyMap[dbPath] = keyHex
			}
			e.derivedKeyMu.Unlock()
			log.Debug().
				Int("matched_dbs", len(matchingDBs)).
				Str("key", keyHex).
				Msg("Derived encKey matched databases")
		}
	}
}

// GetDerivedKeyMap returns a copy of the derived key map (dbRelPath -> keyHex).
func (e *V4Extractor) GetDerivedKeyMap() map[string]string {
	e.derivedKeyMu.Lock()
	defer e.derivedKeyMu.Unlock()
	result := make(map[string]string, len(e.derivedKeyMap))
	for k, v := range e.derivedKeyMap {
		result[k] = v
	}
	return result
}

func (e *V4Extractor) SetValidate(validator *decrypt.Validator) {
	e.validator = validator
}

type KeyPatternInfo struct {
	Pattern []byte
	Offsets []int
}
