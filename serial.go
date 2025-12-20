package kvm

import (
	"io"

	"github.com/pion/webrtc/v4"
	"go.bug.st/serial"
)

const serialPortPath = "/dev/ttyS3"

var port serial.Port

var defaultMode = &serial.Mode{
	BaudRate: 115200,
	DataBits: 8,
	Parity:   serial.NoParity,
	StopBits: serial.OneStopBit,
}

func initSerialPort() error {
	var err error
	port, err = serial.Open(serialPortPath, defaultMode)
	if err != nil {
		return err
	}
	return nil
}

func reopenSerialPort() error {
	if port != nil {
		port.Close()
	}
	return initSerialPort()
}

func handleSerialChannel(d *webrtc.DataChannel) {
	d.OnMessage(func(msg webrtc.DataChannelMessage) {
		if port == nil {
			return
		}
		_, _ = port.Write(msg.Data)
	})
	d.OnOpen(func() {
		go func() {
			buf := make([]byte, 1024)
			for {
				n, err := port.Read(buf)
				if err != nil {
					if err != io.EOF {
						serialLogger.Warn().Err(err).Msg("Error reading from serial port")
					}
					return
				}
				if n > 0 {
					err = d.Send(buf[:n])
					if err != nil {
						serialLogger.Warn().Err(err).Msg("Error sending data to WebRTC channel")
						return
					}
				}
			}
		}()
	})
}
