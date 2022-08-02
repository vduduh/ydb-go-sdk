package topicsugar

import (
	"context"
	"encoding/json"
	"errors"

	"google.golang.org/protobuf/proto"

	"github.com/ydb-platform/ydb-go-sdk/v3/internal/xcontext"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/xerrors"
	"github.com/ydb-platform/ydb-go-sdk/v3/topic/topicreader"
)

var errProcessBatchCompleted = xerrors.Wrap(errors.New("ydb: batch process completed"))

// ReadProcessAndCommitLoop is a simple way to process messages
// it read message batch from reader, call `process` callback for process message in your code, then commit message
// if process return nil
//
// ReadProcessAndCommitLoop work until ctx cancelled
// return nil if ctx cancelled else - return read/process/commit error
func ReadProcessAndCommitLoop(ctx context.Context, r batchReaderAndCommitter, process func(ctx context.Context, batch *topicreader.Batch) error) error {
	iteration := func() error {
		batch, err := r.ReadMessageBatch(ctx)
		if err != nil {
			return err
		}

		processCtx, cancel := xcontext.WithErrCancel(ctx)
		defer cancel(errProcessBatchCompleted)

		go func() {
			select {
			case <-batch.Context().Done():
				cancel(batch.Context().Err())
			case <-processCtx.Done():
				// pass
			}
		}()

		if err = process(processCtx, batch); err != nil {
			return err
		}

		return r.Commit(ctx, batch)
	}

	for {
		if err := iteration(); err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return err
		}
	}
}

type batchReaderAndCommitter interface {
	ReadMessageBatch(ctx context.Context, opts ...topicreader.ReadBatchOption) (*topicreader.Batch, error)
	Commit(ctx context.Context, obj topicreader.CommitRangeGetter) error
}

// ProtoUnmarshal unmarshal message content to protobuf struct
//
// Experimental
//
// Notice: This API is EXPERIMENTAL and may be changed or removed in a later release.
func ProtoUnmarshal(msg *topicreader.Message, dst proto.Message) error {
	return msg.UnmarshalTo(protobufUnmarshaler{dst: dst})
}

// JSONUnmarshal unmarshal json message content to dst
// dst must by pointer to struct
//
// Experimental
//
// Notice: This API is EXPERIMENTAL and may be changed or removed in a later release.
func JSONUnmarshal(msg *topicreader.Message, dst interface{}) error {
	return msg.UnmarshalTo(UnmarshalMessageWith(json.Unmarshal, dst))
}

// UnmarshalMessageWith call unmarshaller func with message content
// unmarshaller func must not use received byte slice after return.
//
// Experimental
//
// Notice: This API is EXPERIMENTAL and may be changed or removed in a later release.
func UnmarshalMessageWith(unmarshaler UnmarshalFunc, v interface{}) topicreader.MessageContentUnmarshaler {
	return messageUnmarshaler{unmarshaler: unmarshaler, dst: v}
}

// ConsumeWithCallback receive full content of message as data
// data slice MUST not be used after return from f.
// if you need content after return from function - copy it with
// copy(dst, data) to another byte slice
//
// Experimental
//
// Notice: This API is EXPERIMENTAL and may be changed or removed in a later release.
func ConsumeWithCallback(f func(data []byte) error) topicreader.MessageContentUnmarshaler {
	return callbackConsumer(f)
}

type callbackConsumer func(data []byte) error

// UnmarshalYDBTopicMessage unmarshal implementation
//
// Experimental
//
// Notice: This API is EXPERIMENTAL and may be changed or removed in a later release.
func (c callbackConsumer) UnmarshalYDBTopicMessage(data []byte) error {
	return c(data)
}

// UnmarshalFunc is func to unmarshal data to interface, for example
// json.Unmarshal from standard library
//
// Experimental
//
// Notice: This API is EXPERIMENTAL and may be changed or removed in a later release.
type UnmarshalFunc func(data []byte, dst interface{}) error

type protobufUnmarshaler struct {
	dst proto.Message
}

// UnmarshalYDBTopicMessage implement unmarshaller
//
// Experimental
//
// Notice: This API is EXPERIMENTAL and may be changed or removed in a later release.
func (m protobufUnmarshaler) UnmarshalYDBTopicMessage(data []byte) error {
	return proto.Unmarshal(data, m.dst)
}

type messageUnmarshaler struct {
	unmarshaler UnmarshalFunc
	dst         interface{}
}

// UnmarshalYDBTopicMessage
//
// Experimental
//
// Notice: This API is EXPERIMENTAL and may be changed or removed in a later release.
func (m messageUnmarshaler) UnmarshalYDBTopicMessage(data []byte) error {
	return m.unmarshaler(data, m.dst)
}
