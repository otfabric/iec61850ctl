package server

import (
	"context"
	"errors"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
)

// TestStartReadOnlyFCWrites: IEC 61850-7-2 makes status values (FC ST)
// read-only to clients, so the server refuses a client write unless
// RunConfig.AllowReadOnlyFCWrites (--allow-readonly-fc-writes) is set.
func TestStartReadOnlyFCWrites(t *testing.T) {
	t.Parallel()
	for _, allow := range []bool{false, true} {
		r, err := Start(context.Background(), RunConfig{
			SclPath:               interopICD(t),
			Host:                  "127.0.0.1",
			AllowReadOnlyFCWrites: allow,
		})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		c, err := iec61850.Dial(ctx, r.Addr(), iec61850.DialOptions{})
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		ref, err := iec61850.ParseRef("InteropLD/GGIO1.SPS1.stVal[ST]")
		if err != nil {
			t.Fatal(err)
		}
		werr := c.Write(ctx, ref, mms.NewBoolean(true))
		v, rerr := c.Read(ctx, ref)
		_ = c.Close(ctx)
		cancel()
		_ = r.Close()
		if rerr != nil {
			t.Fatalf("allow=%v: read back: %v", allow, rerr)
		}
		got, _ := v.Bool()
		if allow {
			if werr != nil || !got {
				t.Fatalf("allow=true: write err=%v, stVal=%v; want accepted and true", werr, got)
			}
			continue
		}
		var de *mms.DataAccessError
		if !errors.As(werr, &de) || de.Code != mms.DataAccessErrorObjectAccessDenied {
			t.Fatalf("allow=false: write err=%v; want object-access-denied", werr)
		}
		if got {
			t.Fatal("allow=false: stVal changed to true by a refused write")
		}
	}
}
