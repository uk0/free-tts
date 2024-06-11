package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/faiface/beep/mp3"
	"github.com/hajimehoshi/oto"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
	"net/url"
	"os"
)

func main() {
	speaker("ChatTTS is a text-to-speech model designed specifically for dialogue scenario such as LLM assistant. It supports both English and Chinese languages. Our model is trained with 100,000+ hours composed of chinese and english. The open-source version on HuggingFace is a 40,000 hours pre trained model without SFT.")
}

func speaker(txt string) {
	// Define API endpoint and parameters
	url1 := "http://127.0.0.1:8080/api/tts"
	params := `?thread=10&shardLength=400&text=%s`
	// 对 txt 进行 URL 编码
	encodedTxt := url.QueryEscape(txt)

	// 构建完整的 URL
	fullURL := url1 + fmt.Sprintf(params, encodedTxt)
	// Send HTTP GET request
	resp, err := http.Get(fullURL)
	if err != nil {
		fmt.Println("Error making GET request:", err)
		return
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		fmt.Println("Received non-OK HTTP status:", resp.Status)
		return
	}

	// Read response content into a buffer
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	// Decode MP3 data using beep
	streamer, format, err := mp3.Decode(io.NopCloser(buf))
	if err != nil {
		fmt.Println("Error decoding MP3:", err)
		return
	}
	defer streamer.Close()
	// Initialize oto player
	ctx, err := oto.NewContext(int(format.SampleRate), format.NumChannels, 2, 4096)
	if err != nil {
		fmt.Println("Error initializing Oto context:", err)
		return
	}
	defer ctx.Close()

	player := ctx.NewPlayer()
	defer player.Close()

	// Create output file
	outputFile, err := os.Create("output.wav")
	if err != nil {
		fmt.Println("Error creating output file:", err)
		return
	}
	defer outputFile.Close()

	// Write WAV header
	writeWavHeader(outputFile, int(format.SampleRate), format.NumChannels)

	// Play the stream
	eg := errgroup.Group{}
	eg.Go(func() error {
		buffer := make([][2]float64, 4096)
		totalBytes := 0
		for {
			n, ok := streamer.Stream(buffer)
			if !ok {
				break
			}
			data := make([]byte, n*4)
			for i := 0; i < n; i++ {
				sample := buffer[i]
				left := int16(sample[0] * 32767)
				right := int16(sample[1] * 32767)
				binary.LittleEndian.PutUint16(data[i*4:], uint16(left))
				binary.LittleEndian.PutUint16(data[i*4+2:], uint16(right))
			}
			_, err := player.Write(data)
			if err != nil {
				return err
			}
			_, err = outputFile.Write(data)
			if err != nil {
				return err
			}
			totalBytes += len(data)
		}
		// Update WAV header with correct file sizes
		return updateWavHeader(outputFile, totalBytes)
	})
	if err := eg.Wait(); err != nil {
		fmt.Println("Playback error:", err)
	} else {
		fmt.Println("Audio playback finished")
	}
}

func writeWavHeader(w io.Writer, sampleRate int, numChannels int) {
	bitsPerSample := 16
	writeString(w, "RIFF")
	writeUint32(w, 0) // Placeholder for file size
	writeString(w, "WAVE")
	writeString(w, "fmt ")
	writeUint32(w, 16) // fmt chunk size
	writeUint16(w, 1)  // audio format (1 = PCM)
	writeUint16(w, uint16(numChannels))
	writeUint32(w, uint32(sampleRate))
	writeUint32(w, uint32(sampleRate*numChannels*bitsPerSample/8)) // byte rate
	writeUint16(w, uint16(numChannels*bitsPerSample/8))            // block align
	writeUint16(w, uint16(bitsPerSample))
	writeString(w, "data")
	writeUint32(w, 0) // Placeholder for data chunk size
}

func updateWavHeader(w io.WriteSeeker, totalBytes int) error {
	fileSize := 36 + totalBytes
	_, err := w.Seek(4, 0)
	if err != nil {
		return err
	}
	writeUint32(w, uint32(fileSize))
	_, err = w.Seek(40, 0)
	if err != nil {
		return err
	}
	writeUint32(w, uint32(totalBytes))
	return nil
}

func writeString(w io.Writer, s string) {
	_, _ = w.Write([]byte(s))
}

func writeUint32(w io.Writer, v uint32) {
	_ = binary.Write(w, binary.LittleEndian, v)
}

func writeUint16(w io.Writer, v uint16) {
	_ = binary.Write(w, binary.LittleEndian, v)
}
