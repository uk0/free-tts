package main

import (
	"bytes"
	"fmt"
	"github.com/faiface/beep/mp3"
	"github.com/hajimehoshi/oto"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
	"net/url"
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

	// Play the stream
	eg := errgroup.Group{}
	eg.Go(func() error {
		buffer := make([][2]float64, 4096)
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
				data[i*4] = byte(left)
				data[i*4+1] = byte(left >> 8)
				data[i*4+2] = byte(right)
				data[i*4+3] = byte(right >> 8)
			}
			_, err := player.Write(data)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		fmt.Println("Playback error:", err)
	} else {
		fmt.Println("Audio playback finished")
	}
}
