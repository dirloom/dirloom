package gitsource

import "context"

// Session is one conversation with Git for one request: a version-checked
// runner, the repository's object format, and at most one `cat-file --batch`
// process, started at the first object read, that serves every object the
// request reads. It is used by a single goroutine; cancellation of its
// context ends the batch process.
type Session struct {
	ctx    context.Context
	runner *Runner
	format ObjectFormat
	batch  *Batch
}

// OpenSession checks git and reads the object format of the repository Git
// discovers from dir (empty means the process working directory). ctx bounds
// every command of the session, the batch process included.
func OpenSession(ctx context.Context, dir string) (*Session, error) {
	runner, err := NewRunner(ctx, dir)
	if err != nil {
		return nil, err
	}
	format, err := runner.objectFormat(ctx)
	if err != nil {
		return nil, err
	}
	return &Session{ctx: ctx, runner: runner, format: format}, nil
}

// Format returns the repository's object format.
func (s *Session) Format() ObjectFormat { return s.format }

// Resolve turns a revision into the tree to observe (Git Tree Source v1,
// Resolution). The revision must already satisfy ValidateRevision.
func (s *Session) Resolve(ctx context.Context, revision string) (Resolved, error) {
	return s.runner.resolveIn(ctx, revision, s.format)
}

// Backend returns a tree backend that reads through the session's batch
// process, starting it if needed.
func (s *Session) Backend() (*Backend, error) {
	batch, err := s.objects()
	if err != nil {
		return nil, err
	}
	return NewBackend(batch), nil
}

// objects returns the batch process, started at the first call.
func (s *Session) objects() (*Batch, error) {
	if s.batch == nil {
		batch, err := s.runner.StartBatch(s.ctx, s.format)
		if err != nil {
			return nil, err
		}
		s.batch = batch
	}
	return s.batch, nil
}

// Close ends the batch process if one was started; see Batch.Close.
func (s *Session) Close() error {
	if s.batch == nil {
		return nil
	}
	return s.batch.Close()
}
