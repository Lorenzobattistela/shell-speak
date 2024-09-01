package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	"github.com/gordonklaus/portaudio"
)

func main() {
	// Initialize PortAudio
	portaudio.Initialize()
	defer portaudio.Terminate()

	// Get available audio devices
	inputDevices, outputDevices := getAvailableDevices()

	inputDevice := selectDevice(inputDevices)
	outputDevice := selectDevice(outputDevices)

	// Define buffer size and sample rate
	const bufferSize = 256
	const sampleRate = 44100

	// Create input and output streams
	inStream, outStream, in, out := createStreams(inputDevice, outputDevice, bufferSize, sampleRate)
	defer inStream.Close()
	defer outStream.Close()

	// Start the streams
	err := inStream.Start()
	check(err)
	defer inStream.Stop()

	err = outStream.Start()
	check(err)
	defer outStream.Stop()

	fmt.Println("Recording and playing audio. Press Ctrl-C to stop.")

	// Record and play audio
	for {
		// Read from input stream
		err := inStream.Read()
		if err != nil {
			fmt.Println("Error reading from input stream:", err)
			continue
		}

		// Copy input buffer to output buffer
		copy(out, in)

		// Write to output stream
		err = outStream.Write()
		if err != nil {
			fmt.Println("Error writing to output stream:", err)
		}
	}
}

func getAvailableDevices() ([]*portaudio.DeviceInfo, []*portaudio.DeviceInfo) {
	devices, err := portaudio.Devices()
	if err != nil {
		panic(err)
	}

	inputDevices := make([]*portaudio.DeviceInfo, 0)
	outputDevices := make([]*portaudio.DeviceInfo, 0)

	for _, device := range devices {
		if device.MaxInputChannels > 0 {
			inputDevices = append(inputDevices, device)
		}

		if device.MaxOutputChannels > 0 {
			outputDevices = append(outputDevices, device)
		}
	}

	return inputDevices, outputDevices
}

func selectDevice(devices []*portaudio.DeviceInfo) *portaudio.DeviceInfo {
	fmt.Println("Available devices:")
	for i, device := range devices {
		fmt.Printf("%d: %s\n", i, device.Name)
	}

	fmt.Print("Enter the index of the device to use: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	text := scanner.Text()

	index, err := strconv.Atoi(text)
	check(err)

	if index < 0 || index >= len(devices) {
		panic("Invalid device index")
	}

	return devices[index]
}

func createStreams(inputDevice *portaudio.DeviceInfo, outputDevice *portaudio.DeviceInfo, bufferSize int, sampleRate float64) (*portaudio.Stream, *portaudio.Stream, []int16, []int16) {
	// Create buffers for input and output
	in := make([]int16, bufferSize)
	out := make([]int16, bufferSize)

	// Open input stream for recording
	inStream, err := portaudio.OpenStream(portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   inputDevice,
			Channels: inputDevice.MaxInputChannels,
			Latency:  inputDevice.DefaultLowInputLatency,
		},
		SampleRate:      sampleRate,
		FramesPerBuffer: bufferSize,
	}, in)
	check(err)

	// Open output stream for playback
	// outStream, err := portaudio.OpenStream(portaudio.StreamParameters{
	// 	Output: portaudio.StreamDeviceParameters{
	// 		Device:   outputDevice,
	// 		Channels: outputDevice.MaxOutputChannels,
	// 		Latency:  outputDevice.DefaultLowOutputLatency,
	// 	},
	// 	SampleRate:      sampleRate,
	// 	FramesPerBuffer: bufferSize,
	// }, out)

	outStream, err := portaudio.OpenDefaultStream(0, 1, sampleRate, bufferSize, out)

	check(err)

	return inStream, outStream, in, out
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
