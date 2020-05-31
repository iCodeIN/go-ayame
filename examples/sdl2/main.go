package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/hakobera/go-ayame/ayame"
	"github.com/hakobera/go-ayame/pkg/decoder"
	"github.com/hakobera/go-ayame/pkg/decoder/vpx"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v2"
	"github.com/veandco/go-sdl2/sdl"
)

const WindowWidth = 640
const WindowHeight = 480

func main() {
	signalingURL := flag.String("url", "wss://ayame-lite.shiguredo.jp/signaling", "Specify Ayame service address")
	roomID := flag.String("room-id", "", "specify room ID")
	signalingKey := flag.String("signaling-key", "", "specify signaling key")
	verbose := flag.Bool("verbose", false, "enable verbose log")

	flag.Parse()
	log.Printf("args: url=%s, roomID=%s, signalingKey=%s", *signalingURL, *roomID, *signalingKey)

	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		log.Printf("Failed to initialize SDL")
		panic(err)
	}
	defer sdl.Quit()

	window, err := sdl.CreateWindow("go-ayame SDL2 example", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, WindowWidth, WindowHeight, sdl.WINDOW_SHOWN|sdl.WINDOW_ALLOW_HIGHDPI)
	if err != nil {
		log.Printf("Failed to create SDL window")
		panic(err)
	}
	defer window.Destroy()

	renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		log.Printf("Failed to create SDL renderer")
		panic(err)
	}
	defer renderer.Destroy()

	texture, err := renderer.CreateTexture(sdl.PIXELFORMAT_YV12, sdl.TEXTUREACCESS_STREAMING, WindowWidth, WindowHeight)
	if err != nil {
		log.Printf("Failed to create SDL texture")
		panic(err)
	}
	defer texture.Destroy()

	renderer.SetDrawColor(0, 0, 0, sdl.ALPHA_OPAQUE)
	renderer.Clear()

	opts := ayame.DefaultOptions()
	opts.SignalingKey = *signalingKey
	opts.Audio.Enabled = false

	d, err := vpx.NewVP8Decoder()
	if err != nil {
		log.Printf("Failed to create VideoDecoder")
		panic(err)
	}
	defer d.Close()
	videoFrameBuilder := d.NewFrameBuilder()

	videoData := make(chan *decoder.Frame, 60)
	defer close(videoData)

	imgData := make(chan vpx.DecodedImage)

	go d.Process(videoData, imgData)

	con := ayame.NewConnection(*signalingURL, *roomID, opts, *verbose, false)
	defer con.Disconnect()

	con.OnConnect(func() {
		fmt.Println("Connected")
	})
	con.OnBye(func() {
		fmt.Printf("Disconnected by peer. Press Ctrl+C to exit.\n")
	})

	con.OnTrackPacket(func(track *webrtc.Track, packet *rtp.Packet) {
		switch track.Kind() {
		case webrtc.RTPCodecTypeVideo:
			videoFrameBuilder.Push(packet)

			for {
				frame := videoFrameBuilder.Pop()
				if frame == nil {
					return
				}
				videoData <- frame
			}
		}
	})

	err = con.Connect()
	if err != nil {
		log.Fatal("failed to connect Ayame", err)
	}

	go func() {
		for {
			var err error = nil
			select {
			case img, ok := <-imgData:
				if !ok {
					return
				}

				err = texture.UpdateYUV(nil, img.Plane(0), img.Stride(0), img.Plane(1), img.Stride(1), img.Plane(2), img.Stride(2))
				if err != nil {
					log.Println("Failed to update SDL Texture", err)
					continue
				}

				src := &sdl.Rect{0, 0, int32(img.Width()), int32(img.Height())}
				dst := &sdl.Rect{0, 0, WindowWidth, WindowHeight}

				renderer.Clear()
				renderer.Copy(texture, src, dst)
				renderer.Present()
			}
		}
	}()

	running := true
	for running {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event.(type) {
			case *sdl.QuitEvent:
				println("Quit")
				running = false
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}
