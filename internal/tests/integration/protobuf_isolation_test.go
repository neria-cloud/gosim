//go:build sim

package behavior_test

import (
	"fmt"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
)

// TestProtobufStatusPerMachine resolves the same real status and detail
// descriptors in two machines. The native metatest repeats this test within
// one worker after intervening seeds to catch leaked translated globals.
func TestProtobufStatusPerMachine(t *testing.T) {
	fingerprints := make(chan string, 2)
	for i := 0; i < 2; i++ {
		m := testsupport.Start(t, func() {
			base := status.New(codes.InvalidArgument, "bad field")
			withDetails, err := base.WithDetails(&errdetails.BadRequest{
				FieldViolations: []*errdetails.BadRequest_FieldViolation{{Field: "code", Description: "invalid"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			parsed, ok := status.FromError(withDetails.Err())
			if !ok || parsed.Code() != codes.InvalidArgument || len(parsed.Details()) != 1 {
				t.Fatalf("status/detail round trip: %+v, %v", parsed, ok)
			}
			if detail, ok := parsed.Details()[0].(*errdetails.BadRequest); !ok || len(detail.FieldViolations) != 1 || detail.FieldViolations[0].Field != "code" {
				t.Fatalf("detail changed: %+v", parsed.Details())
			}
			message := parsed.Proto()
			descriptor := message.ProtoReflect().Descriptor()
			field := descriptor.Fields().ByName(protoreflect.Name("code"))
			if descriptor.FullName() != "google.rpc.Status" || field == nil || field.Number() != 1 {
				t.Fatalf("status descriptor changed: %s, %+v", descriptor.FullName(), field)
			}
			encoded, err := protojson.Marshal(message)
			if err != nil {
				t.Fatal(err)
			}
			fingerprints <- fmt.Sprintf("%s|%s", descriptor.FullName(), encoded)
		})
		if exit := testsupport.Wait(t, m); exit.Kind != gosim.Graceful {
			t.Fatalf("descriptor machine exited unexpectedly: %+v", exit)
		}
	}
	first, second := <-fingerprints, <-fingerprints
	if first == "" || first != second {
		t.Fatalf("machine-local descriptor results differ: %q, %q", first, second)
	}
}
