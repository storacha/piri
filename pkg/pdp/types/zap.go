package types

import (
	"github.com/google/uuid"
	"go.uber.org/zap/zapcore"
)

// MarshalLogObject implements zapcore.ObjectMarshaler for ProofSetStatus
func (p ProofSetStatus) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("tx_hash", p.TxHash.Hex())
	enc.AddString("tx_status", p.TxStatus)
	enc.AddBool("created", p.Created)
	enc.AddUint64("id", p.ID)
	return nil
}

// MarshalLogObject implements zapcore.ObjectMarshaler for ProofSet
func (p ProofSet) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint64("id", p.ID)
	enc.AddBool("initialized", p.Initialized)
	enc.AddInt64("next_challenge_epoch", p.NextChallengeEpoch)
	enc.AddInt64("previous_challenge_epoch", p.PreviousChallengeEpoch)
	enc.AddInt64("proving_period", p.ProvingPeriod)
	enc.AddInt64("challenge_window", p.ChallengeWindow)
	return enc.AddArray("roots", zapcore.ArrayMarshalerFunc(func(ae zapcore.ArrayEncoder) error {
		for _, root := range p.Roots {
			if err := ae.AppendObject(root); err != nil {
				return err
			}
		}
		return nil
	}))
}

// MarshalLogObject implements zapcore.ObjectMarshaler for RootEntry
func (r RootEntry) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint64("root_id", r.RootID)
	enc.AddString("root_cid", r.RootCID.String())
	enc.AddString("subroot_cid", r.SubrootCID.String())
	enc.AddInt64("subroot_offset", r.SubrootOffset)
	return nil
}

// MarshalLogObject implements zapcore.ObjectMarshaler for RootAdd
func (r RootAdd) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("root", r.Root.String())
	return enc.AddArray("sub_roots", zapcore.ArrayMarshalerFunc(func(ae zapcore.ArrayEncoder) error {
		for _, subroot := range r.SubRoots {
			ae.AppendString(subroot.String())
		}
		return nil
	}))
}

// MarshalLogObject implements zapcore.ObjectMarshaler for PieceAllocation
func (p PieceAllocation) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	if p.Notify != nil {
		enc.AddString("notify", p.Notify.String())
	} else {
		enc.AddString("notify", "")
	}
	return enc.AddObject("piece", p.Piece)
}

// MarshalLogObject implements zapcore.ObjectMarshaler for Piece
func (p Piece) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("name", p.Name)
	enc.AddString("hash", p.Hash)
	enc.AddInt64("size", p.Size)
	return nil
}

// MarshalLogObject implements zapcore.ObjectMarshaler for PieceUpload
func (p PieceUpload) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("id", p.ID.String())
	enc.AddBool("has_data", p.Data != nil)
	return nil
}

// MarshalLogObject implements zapcore.ObjectMarshaler for AllocatedPiece
func (a AllocatedPiece) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddBool("allocated", a.Allocated)
	enc.AddString("piece", a.Piece.String())
	if a.UploadID != uuid.Nil {
		enc.AddString("upload_id", a.UploadID.String())
	}
	return nil
}

// MarshalLogObject implements zapcore.ObjectMarshaler for PieceReader
func (p PieceReader) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("size", p.Size)
	enc.AddBool("has_data", p.Data != nil)
	return nil
}
