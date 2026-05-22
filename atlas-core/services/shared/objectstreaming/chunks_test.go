package objectstreaming

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidateWriteChunkMetadata_ExpectedSizeConsistency(t *testing.T) {
	file := WriteFileMetadata{
		ObjectID:     "obj_001",
		Filename:     "data.bin",
		ExpectedSize: 100,
	}

	if err := ValidateWriteChunkMetadata(file, "obj_001", "data.bin", 100); err != nil {
		t.Fatalf("matching expected_size: %v", err)
	}
	if err := ValidateWriteChunkMetadata(file, "obj_001", "data.bin", 0); err == nil {
		t.Fatal("expected error when later chunk expected_size differs from first")
	} else if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}

	file.ExpectedSize = 0
	if err := ValidateWriteChunkMetadata(file, "obj_001", "data.bin", 0); err != nil {
		t.Fatalf("matching zero expected_size: %v", err)
	}
	if err := ValidateWriteChunkMetadata(file, "obj_001", "data.bin", 100); err == nil {
		t.Fatal("expected error when first chunk expected_size is 0 and later is non-zero")
	}
}
