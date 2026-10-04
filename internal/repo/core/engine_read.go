// Engine read path: index lookups over the live record area.

package core

import (
	"errors"
	"io"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
)

func (e *StorageEngine) ReadRecord(agentID, idHash uint64) (uint8, []byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed {
		return 0, nil, errEngineClosed
	}
	offset, ok := e.index[agentID][idHash]
	if !ok {
		return 0, nil, common.NewError(common.ErrNotFound, "record not found")
	}
	rt, _, data, _, _, err := RecordData(e.mmap, offset)
	if err != nil {
		if errors.Is(err, io.EOF) {
			// io.EOF is the frame scanner's "stop here", which means nothing to a caller that named one id.
			return 0, nil, common.NewError(common.ErrCorruption,
				"the index names a record the record area does not hold", err)
		}
		return 0, nil, err
	}
	return rt, data, nil
}
