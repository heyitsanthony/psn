package psn

import (
	"encoding/csv"
	"fmt"
	"strconv"

	humanize "github.com/dustin/go-humanize"
)

// CSV represents CSV data (header, rows, etc.).
type CSV struct {
	FilePath         string
	PID              int64
	DiskDevice       string
	NetworkInterface string

	Header      []string
	HeaderIndex map[string]int

	MinUnixTS int64
	MaxUnixTS int64

	// ExtraPath contains extra information.
	ExtraPath string

	// Rows are sorted by unix seconds.
	Rows []Proc
}

// NewCSV returns a new CSV.
func NewCSV(fpath string, pid int64, diskDevice string, networkInterface string, extraPath string) *CSV {
	return &CSV{
		FilePath:         fpath,
		PID:              pid,
		DiskDevice:       diskDevice,
		NetworkInterface: networkInterface,

		Header:      ProcHeader,
		HeaderIndex: ProcHeaderIndex,

		MinUnixTS: 0,
		MaxUnixTS: 0,

		ExtraPath: extraPath,
		Rows:      []Proc{},
	}
}

// Add is called periodically to append a new entry to CSV; it only appends.
// If the data is used for time series, make sure to handle missing time stamps between.
// e.g. interpolate by estimating the averages between last row and new row to be inserted.
func (c *CSV) Add() error {
	cur, err := GetProc(
		WithPID(c.PID),
		WithDiskDevice(c.DiskDevice),
		WithNetworkInterface(c.NetworkInterface),
		WithExtraPath(c.ExtraPath),
	)
	if err != nil {
		return err
	}

	// first call; just append and return
	if len(c.Rows) == 0 {
		c.MinUnixTS = cur.UnixTS
		c.MaxUnixTS = cur.UnixTS
		c.Rows = []Proc{cur}
		return nil
	}

	// compare with previous row before append
	prev := c.Rows[len(c.Rows)-1]
	if prev.UnixTS >= cur.UnixTS {
		return fmt.Errorf("clock went backwards: got %v, but expected more than %v", cur.UnixTS, prev.UnixTS)
	}

	// 'Add' only appends, so later unix should be max
	c.MaxUnixTS = cur.UnixTS

	cur.ReadsCompletedDelta = cur.DSEntry.ReadsCompleted - prev.DSEntry.ReadsCompleted
	cur.SectorsReadDelta = cur.DSEntry.SectorsRead - prev.DSEntry.SectorsRead
	cur.WritesCompletedDelta = cur.DSEntry.WritesCompleted - prev.DSEntry.WritesCompleted
	cur.SectorsWrittenDelta = cur.DSEntry.SectorsWritten - prev.DSEntry.SectorsWritten

	cur.ReceiveBytesNumDelta = cur.NSEntry.ReceiveBytesNum - prev.NSEntry.ReceiveBytesNum
	cur.TransmitBytesNumDelta = cur.NSEntry.TransmitBytesNum - prev.NSEntry.TransmitBytesNum
	cur.ReceivePacketsDelta = cur.NSEntry.ReceivePackets - prev.NSEntry.ReceivePackets
	cur.TransmitPacketsDelta = cur.NSEntry.TransmitPackets - prev.NSEntry.TransmitPackets

	cur.ReceiveBytesDelta = humanize.Bytes(cur.ReceiveBytesNumDelta)
	cur.TransmitBytesDelta = humanize.Bytes(cur.TransmitBytesNumDelta)

	c.Rows = append(c.Rows, cur)
	return nil
}

// Save saves CSV to disk.
func (c *CSV) Save() error {
	f, err := openToAppend(c.FilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	wr := csv.NewWriter(f)
	if err := wr.Write(c.Header); err != nil {
		return err
	}

	rows := make([][]string, len(c.Rows))
	for i, row := range c.Rows {
		rows[i] = row.ToRow()
	}
	if err := wr.WriteAll(rows); err != nil {
		return err
	}

	wr.Flush()
	return wr.Error()
}

// ReadCSV reads a CSV file and convert to 'CSV'.
func ReadCSV(fpath string) (*CSV, error) {
	f, err := openToRead(fpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rd := csv.NewReader(f)

	// in case that rows have Deltaerent number of fields
	rd.FieldsPerRecord = -1

	rows, err := rd.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return nil, fmt.Errorf("expected len(rows)>1, got %d", len(rows))
	}
	if rows[0][0] != "UNIX-TS" {
		return nil, fmt.Errorf("expected header at top, got %+v", rows[0])
	}

	// remove header
	rows = rows[1:len(rows):len(rows)]
	min, err := strconv.ParseInt(rows[0][0], 10, 64)
	if err != nil {
		return nil, err
	}
	max, err := strconv.ParseInt(rows[len(rows)-1][0], 10, 64)
	if err != nil {
		return nil, err
	}
	c := &CSV{
		FilePath:         fpath,
		PID:              0,
		DiskDevice:       "",
		NetworkInterface: "",

		Header:      ProcHeader,
		HeaderIndex: ProcHeaderIndex,
		MinUnixTS:   min,
		MaxUnixTS:   max,

		Rows: make([]Proc, 0, len(rows)),
	}
	for _, row := range rows {
		ts, err := strconv.ParseInt(row[ProcHeaderIndex["UNIX-TS"]], 10, 64)
		if err != nil {
			return nil, err
		}
		pid, err := strconv.ParseInt(row[ProcHeaderIndex["PID"]], 10, 64)
		if err != nil {
			return nil, err
		}
		ppid, err := strconv.ParseInt(row[ProcHeaderIndex["PPID"]], 10, 64)
		if err != nil {
			return nil, err
		}
		fd, err := strconv.ParseUint(row[ProcHeaderIndex["FD"]], 10, 64)
		if err != nil {
			return nil, err
		}
		threads, err := strconv.ParseUint(row[ProcHeaderIndex["THREADS"]], 10, 64)
		if err != nil {
			return nil, err
		}
		volCtxNum, err := strconv.ParseUint(row[ProcHeaderIndex["VOLUNTARY-CTXT-SWITCHES"]], 10, 64)
		if err != nil {
			return nil, err
		}
		nonVolCtxNum, err := strconv.ParseUint(row[ProcHeaderIndex["NON-VOLUNTARY-CTXT-SWITCHES"]], 10, 64)
		if err != nil {
			return nil, err
		}
		cpuNum, err := strconv.ParseFloat(row[ProcHeaderIndex["CPU-NUM"]], 64)
		if err != nil {
			return nil, err
		}
		vmRssNum, err := strconv.ParseUint(row[ProcHeaderIndex["VMRSS-NUM"]], 10, 64)
		if err != nil {
			return nil, err
		}
		vmSizeNum, err := strconv.ParseUint(row[ProcHeaderIndex["VMSIZE-NUM"]], 10, 64)
		if err != nil {
			return nil, err
		}

		readsCompleted, err := strconv.ParseUint(row[ProcHeaderIndex["READS-COMPLETED"]], 10, 64)
		if err != nil {
			return nil, err
		}
		sectorsRead, err := strconv.ParseUint(row[ProcHeaderIndex["SECTORS-READ"]], 10, 64)
		if err != nil {
			return nil, err
		}
		writesCompleted, err := strconv.ParseUint(row[ProcHeaderIndex["WRITES-COMPLETED"]], 10, 64)
		if err != nil {
			return nil, err
		}
		sectorsWritten, err := strconv.ParseUint(row[ProcHeaderIndex["SECTORS-WRITTEN"]], 10, 64)
		if err != nil {
			return nil, err
		}
		timeSpentOnReadingMs, err := strconv.ParseUint(row[ProcHeaderIndex["MILLISECONDS(READS)"]], 10, 64)
		if err != nil {
			return nil, err
		}
		timeSpentOnWritingMs, err := strconv.ParseUint(row[ProcHeaderIndex["MILLISECONDS(WRITES)"]], 10, 64)
		if err != nil {
			return nil, err
		}

		readsCompletedDelta, err := strconv.ParseUint(row[ProcHeaderIndex["READS-COMPLETED-DELTA"]], 10, 64)
		if err != nil {
			return nil, err
		}
		sectorsReadDelta, err := strconv.ParseUint(row[ProcHeaderIndex["SECTORS-READ-DELTA"]], 10, 64)
		if err != nil {
			return nil, err
		}
		writesCompletedDelta, err := strconv.ParseUint(row[ProcHeaderIndex["WRITES-COMPLETED-DELTA"]], 10, 64)
		if err != nil {
			return nil, err
		}
		sectorsWrittenDelta, err := strconv.ParseUint(row[ProcHeaderIndex["SECTORS-WRITTEN-DELTA"]], 10, 64)
		if err != nil {
			return nil, err
		}

		receivePackets, err := strconv.ParseUint(row[ProcHeaderIndex["RECEIVE-PACKETS"]], 10, 64)
		if err != nil {
			return nil, err
		}
		transmitPackets, err := strconv.ParseUint(row[ProcHeaderIndex["TRANSMIT-PACKETS"]], 10, 64)
		if err != nil {
			return nil, err
		}
		receiveBytesNum, err := strconv.ParseUint(row[ProcHeaderIndex["RECEIVE-BYTES-NUM"]], 10, 64)
		if err != nil {
			return nil, err
		}
		transmitBytesNum, err := strconv.ParseUint(row[ProcHeaderIndex["TRANSMIT-BYTES-NUM"]], 10, 64)
		if err != nil {
			return nil, err
		}

		receivePacketsDelta, err := strconv.ParseUint(row[ProcHeaderIndex["RECEIVE-PACKETS-DELTA"]], 10, 64)
		if err != nil {
			return nil, err
		}
		transmitPacketsDelta, err := strconv.ParseUint(row[ProcHeaderIndex["TRANSMIT-PACKETS-DELTA"]], 10, 64)
		if err != nil {
			return nil, err
		}
		receiveBytesNumDelta, err := strconv.ParseUint(row[ProcHeaderIndex["RECEIVE-BYTES-NUM-DELTA"]], 10, 64)
		if err != nil {
			return nil, err
		}
		transmitBytesNumDelta, err := strconv.ParseUint(row[ProcHeaderIndex["TRANSMIT-BYTES-NUM-DELTA"]], 10, 64)
		if err != nil {
			return nil, err
		}

		proc := Proc{
			UnixTS: ts,
			PSEntry: PSEntry{
				Program:                  row[ProcHeaderIndex["PROGRAM"]],
				State:                    row[ProcHeaderIndex["STATE"]],
				PID:                      pid,
				PPID:                     ppid,
				CPU:                      row[ProcHeaderIndex["CPU"]],
				VMRSS:                    row[ProcHeaderIndex["VMRSS"]],
				VMSize:                   row[ProcHeaderIndex["VMSIZE"]],
				FD:                       fd,
				Threads:                  threads,
				VoluntaryCtxtSwitches:    volCtxNum,
				NonvoluntaryCtxtSwitches: nonVolCtxNum,
				CPUNum:    cpuNum,
				VMRSSNum:  vmRssNum,
				VMSizeNum: vmSizeNum,
			},

			DSEntry: DSEntry{
				Device:               row[ProcHeaderIndex["DEVICE"]],
				ReadsCompleted:       readsCompleted,
				SectorsRead:          sectorsRead,
				TimeSpentOnReading:   row[ProcHeaderIndex["TIME(READS)"]],
				WritesCompleted:      writesCompleted,
				SectorsWritten:       sectorsWritten,
				TimeSpentOnWriting:   row[ProcHeaderIndex["TIME(WRITES)"]],
				TimeSpentOnReadingMs: timeSpentOnReadingMs,
				TimeSpentOnWritingMs: timeSpentOnWritingMs,
			},
			ReadsCompletedDelta:  readsCompletedDelta,
			SectorsReadDelta:     sectorsReadDelta,
			WritesCompletedDelta: writesCompletedDelta,
			SectorsWrittenDelta:  sectorsWrittenDelta,

			NSEntry: NSEntry{
				Interface:        row[ProcHeaderIndex["INTERFACE"]],
				ReceiveBytes:     row[ProcHeaderIndex["RECEIVE-BYTES"]],
				ReceivePackets:   receivePackets,
				TransmitBytes:    row[ProcHeaderIndex["TRANSMIT-BYTES"]],
				TransmitPackets:  transmitPackets,
				ReceiveBytesNum:  receiveBytesNum,
				TransmitBytesNum: transmitBytesNum,
			},
			ReceiveBytesDelta:     row[ProcHeaderIndex["RECEIVE-BYTES-DELTA"]],
			ReceivePacketsDelta:   receivePacketsDelta,
			TransmitBytesDelta:    row[ProcHeaderIndex["TRANSMIT-BYTES-DELTA"]],
			TransmitPacketsDelta:  transmitPacketsDelta,
			ReceiveBytesNumDelta:  receiveBytesNumDelta,
			TransmitBytesNumDelta: transmitBytesNumDelta,

			Extra: []byte(row[ProcHeaderIndex["EXTRA"]]),
		}
		c.PID = proc.PSEntry.PID
		c.DiskDevice = proc.DSEntry.Device
		c.NetworkInterface = proc.NSEntry.Interface

		c.Rows = append(c.Rows, proc)
	}

	return c, nil
}
