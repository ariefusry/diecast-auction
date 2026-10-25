package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ariefusry/diecast-auction/internal/testutil"
)

func TestReserveUploadCapsEachUserPerDay(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()
	me, other := e.user(0), e.user(0)
	for i := range UploadsPerUserPerDay {
		if err := e.auctions.ReserveUpload(ctx, me, fmt.Sprintf("%s-%d.jpg", testutil.Unique(), i), 1000); err != nil {
			t.Fatalf("upload %d: %v", i, err)
		}
	}
	if err := e.auctions.ReserveUpload(ctx, me, testutil.Unique()+".jpg", 1000); !errors.Is(err, ErrConflict) {
		t.Fatalf("over the cap: err = %v, want ErrConflict", err)
	}
	// The cap is per user: someone else still uploads.
	if err := e.auctions.ReserveUpload(ctx, other, testutil.Unique()+".jpg", 1000); err != nil {
		t.Fatalf("other user: %v", err)
	}
}

// Fresh accounts are cheap, so a byte budget for everyone guards the disk too.
func TestReserveUploadCapsTotalBytesPerDay(t *testing.T) {
	e := newAuctionEnv(t)
	if err := e.auctions.ReserveUpload(context.Background(), e.user(0), testutil.Unique()+".jpg", UploadBytesPerDay+1); !errors.Is(err, ErrConflict) {
		t.Fatalf("over the byte budget: err = %v, want ErrConflict", err)
	}
}
