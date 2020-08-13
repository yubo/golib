package status

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	epb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewf(t *testing.T) {
	if es, ok := status.FromError(Newf(codes.InvalidArgument, "hello %s", "world").Err()); ok {
		details := es.Details()
		for i := range details {
			{
				out := &epb.DebugInfo{}
				msg := details[i].(proto.Message).String()
				if err := proto.UnmarshalText(msg, out); err != nil {
					t.Fatal(t)
				}

				t.Logf("msg unmarshal %s", out.Detail)
			}

			{
				info := details[i].(*epb.DebugInfo)
				t.Logf("direct %s", info.Detail)
			}

		}
	}
}
