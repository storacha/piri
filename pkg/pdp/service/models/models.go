package models

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

//go:embed triggers.sqlite.sql
var Triggers string

// Task represents the task table.
type Task struct {
	ID           int64     `gorm:"primaryKey;autoIncrement;column:id"`
	InitiatedBy  *int      `gorm:"column:initiated_by;comment:The task ID whose completion occasioned this task"`
	UpdateTime   time.Time `gorm:"not null;default:current_timestamp;column:update_time;comment:When it was last modified. not a heartbeat"`
	PostedTime   time.Time `gorm:"not null;column:posted_time"`
	SessionID    *string   `gorm:"column:session_id;comment:may be null if not yet taken"`
	AddedBy      string    `gorm:"not null;column:added_by"`
	PreviousTask *int      `gorm:"column:previous_task"`
	Name         string    `gorm:"size:16;not null;column:name;comment:The name of the task type"`
	Retries      uint      `gorm:"not null;column:retries"`
	// Note: The "retries" field was commented out in the original schema.

	// TODO consider adding this in when/if we allow more machines
	// OwnerMachine *Machine  `gorm:"foreignKey:OwnerID;references:ID;constraint:OnDelete:SET NULL"` // matches "owner_id references machines(id) on delete set null"
}

func (Task) TableName() string {
	return "task"
}

// TaskHistory represents the task_history table.
type TaskHistory struct {
	ID                   int64     `gorm:"primaryKey;autoIncrement;column:id"`
	TaskID               int64     `gorm:"not null;column:task_id"`
	Name                 string    `gorm:"size:16;not null;column:name"`
	Posted               time.Time `gorm:"not null;column:posted"`
	WorkStart            time.Time `gorm:"not null;column:work_start"`
	WorkEnd              time.Time `gorm:"not null;column:work_end"`
	Result               bool      `gorm:"not null;column:result;comment:Use to determine if this was a successful run."`
	Err                  string    `gorm:"column:err"`
	CompletedBySessionID string    `gorm:"size:300;not null;column:completed_by_session_id"`
}

func (TaskHistory) TableName() string {
	return "task_history"
}

// ParkedPiece represents the parked_pieces table.
type ParkedPiece struct {
	ID              int64     `gorm:"primaryKey;autoIncrement;column:id"`
	CreatedAt       time.Time `gorm:"default:current_timestamp;column:created_at"`
	PieceCID        string    `gorm:"not null;column:piece_cid;uniqueIndex:idx_piece_cid_padded_longterm_cleanup"`
	PiecePaddedSize int64     `gorm:"not null;column:piece_padded_size;uniqueIndex:idx_piece_cid_padded_longterm_cleanup"`
	PieceRawSize    int64     `gorm:"not null;column:piece_raw_size"`
	Complete        bool      `gorm:"not null;default:false;column:complete"`

	TaskID *int64 `gorm:"column:task_id"`
	Task   *Task  `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:SET NULL"` // "task_id references task(id) on delete set null"

	CleanupTaskID *int64 `gorm:"column:cleanup_task_id;uniqueIndex:idx_piece_cid_padded_longterm_cleanup"`
	CleanupTask   *Task  `gorm:"foreignKey:CleanupTaskID;references:ID;constraint:OnDelete:SET NULL"` // "cleanup_task_id references task(id) on delete set null"

	LongTerm bool `gorm:"not null;default:false;column:long_term;uniqueIndex:idx_piece_cid_padded_longterm_cleanup"`
}

func (ParkedPiece) TableName() string {
	return "parked_pieces"
}

// ParkedPieceRef represents the parked_piece_refs table.
type ParkedPieceRef struct {
	RefID       int64        `gorm:"primaryKey;autoIncrement;column:ref_id"`
	PieceID     int64        `gorm:"not null;column:piece_id"`
	ParkedPiece *ParkedPiece `gorm:"foreignKey:PieceID;references:ID;constraint:OnDelete:CASCADE"` // "piece_id references parked_pieces(id) on delete cascade"

	DataURL     string         `gorm:"column:data_url"`
	DataHeaders datatypes.JSON `gorm:"not null;column:data_headers;default:'{}'"`
	LongTerm    bool           `gorm:"not null;default:false;column:long_term"`
}

func (ParkedPieceRef) TableName() string {
	return "parked_piece_refs"
}

// pdp_piece_uploads
type PDPPieceUpload struct {
	ID      string `gorm:"primaryKey;type:uuid"` // or use a UUID type
	Service string `gorm:"not null"`             // references pdp_services(service_label)
	//ServiceModel   *PDPService `gorm:"foreignKey:Service;references:ServiceLabel;constraint:OnDelete:CASCADE"` // "service references pdp_services(service_label) on delete cascade"

	CheckHashCodec string          `gorm:"not null"`
	CheckHash      []byte          `gorm:"not null"`
	CheckSize      int64           `gorm:"not null"`
	PieceCID       *string         `gorm:"column:piece_cid"`
	NotifyURL      string          `gorm:"not null"`
	NotifyTaskID   *int64          // references task(id)
	PieceRef       *int64          // references parked_piece_refs(ref_id)
	ParkedPieceRef *ParkedPieceRef `gorm:"foreignKey:PieceRef;references:RefID;constraint:OnDelete:SET NULL"` // "piece_ref references parked_piece_refs(ref_id) on delete set null"

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;not null"`
}

func (PDPPieceUpload) TableName() string {
	return "pdp_piece_uploads"
}

// pdp_piecerefs
type PDPPieceRef struct {
	ID      int64  `gorm:"primaryKey;autoIncrement"`
	Service string `gorm:"not null"` // references pdp_services(service_label)

	PieceCID       string          `gorm:"not null;column:piece_cid"`
	PieceRef       int64           // references parked_piece_refs(ref_id)
	ParkedPieceRef *ParkedPieceRef `gorm:"foreignKey:PieceRef;references:RefID;constraint:OnDelete:CASCADE"` // "piece_ref references parked_piece_refs(ref_id) on delete cascade"

	CreatedAt        time.Time `gorm:"default:CURRENT_TIMESTAMP;not null"`
	ProofsetRefcount int64     `gorm:"default:0;not null"`
}

func (PDPPieceRef) TableName() string {
	return "pdp_piecerefs"
}

// pdp_piece_mh_to_commp
type PDPPieceMHToCommp struct {
	Mhash []byte `gorm:"primaryKey"` // BYTEA primary key
	Size  int64  `gorm:"not null"`
	Commp string `gorm:"not null"`
}

func (PDPPieceMHToCommp) TableName() string {
	return "pdp_piece_mh_to_commp"
}

// pdp_proof_sets
type PDPProofSet struct {
	ID                        int64 `gorm:"primaryKey"` // big int
	PrevChallengeRequestEpoch *int64
	ChallengeRequestTaskID    *int64 `gorm:"column:challenge_request_task_id"`
	ChallengeRequestTask      *Task  `gorm:"foreignKey:ChallengeRequestTaskID;references:ID;constraint:OnDelete:SET NULL"`

	ChallengeRequestMsgHash *string
	ProvingPeriod           *int64
	ChallengeWindow         *int64
	ProveAtEpoch            *int64
	InitReady               bool   `gorm:"default:false;not null"`
	CreateMessageHash       string `gorm:"not null"`
	Service                 string `gorm:"not null"` // references pdp_services(service_label)
	// ServiceModel *PDPService `gorm:"foreignKey:Service;references:ServiceLabel;constraint:OnDelete:RESTRICT"`
}

func (PDPProofSet) TableName() string {
	return "pdp_proof_sets"
}

// pdp_prove_tasks (composite PK)
type PDPProveTask struct {
	ProofsetID int64 `gorm:"primaryKey"` // references pdp_proof_sets(id)
	TaskID     int64 `gorm:"primaryKey"` // references harmony_task(id)

	ProofSet  *PDPProofSet `gorm:"foreignKey:ProofsetID;references:ID;constraint:OnDelete:CASCADE"` // "proofset references pdp_proof_sets(id) on delete cascade"
	ProveTask *Task        `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`     // "task_id references task(id) on delete cascade"
}

func (PDPProveTask) TableName() string {
	return "pdp_prove_tasks"
}

// pdp_proofset_creates
type PDPProofsetCreate struct {
	CreateMessageHash string           `gorm:"primaryKey"` // references message_waits_eth(signed_tx_hash)
	MessageWait       *MessageWaitsEth `gorm:"foreignKey:CreateMessageHash;references:SignedTxHash;constraint:OnDelete:CASCADE"`

	Ok              *bool  // NULL / TRUE / FALSE
	ProofsetCreated bool   `gorm:"default:false;not null"`
	Service         string `gorm:"not null"` // references pdp_services(service_label)
	//ServiceModel    *PDPService `gorm:"foreignKey:Service;references:ServiceLabel;constraint:OnDelete:CASCADE"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;not null"`
}

func (PDPProofsetCreate) TableName() string {
	return "pdp_proofset_creates"
}

// pdp_proofset_roots (composite PK)
type PDPProofsetRoot struct {
	ProofsetID int64        `gorm:"primaryKey"`                                                      // references pdp_proof_sets(id)
	ProofSet   *PDPProofSet `gorm:"foreignKey:ProofsetID;references:ID;constraint:OnDelete:CASCADE"` // "proofset references pdp_proof_sets(id) on delete cascade"

	RootID         int64            `gorm:"primaryKey"`
	SubrootOffset  int64            `gorm:"primaryKey"`
	Root           string           `gorm:"not null"`
	AddMessageHash string           `gorm:"not null"`                                                                      // references message_waits_eth(signed_tx_hash)
	AddMessageWait *MessageWaitsEth `gorm:"foreignKey:AddMessageHash;references:SignedTxHash;constraint:OnDelete:CASCADE"` // "add_message_hash references message_waits_eth(signed_tx_hash) on delete cascade"

	AddMessageIndex  int64
	Subroot          string `gorm:"not null"`
	SubrootOffsetVal int64  // same as SubrootOffset, but for clarity if needed
	SubrootSize      int64

	PDPPieceRefID *int64       // references pdp_piecerefs(id)
	PDPPieceRef   *PDPPieceRef `gorm:"foreignKey:PDPPieceRefID;references:ID;constraint:OnDelete:SET NULL"` // "pdp_pieceref references pdp_piecerefs(id) on delete set null"

}

func (PDPProofsetRoot) TableName() string {
	return "pdp_proofset_roots"
}

// pdp_proofset_root_adds (composite PK)
type PDPProofsetRootAdd struct {
	ProofsetID int64        `gorm:"primaryKey;column:proofset_id"`                                   // references pdp_proof_sets(id)
	ProofSet   *PDPProofSet `gorm:"foreignKey:ProofsetID;references:ID;constraint:OnDelete:CASCADE"` // "proofset references pdp_proof_sets(id) on delete cascade"

	AddMessageHash string           `gorm:"column:add_message_hash"` // references message_waits_eth(signed_tx_hash)
	AddMessageWait *MessageWaitsEth `gorm:"foreignKey:AddMessageHash;references:SignedTxHash;constraint:OnDelete:CASCADE"`

	SubrootOffset   int64  `gorm:"primaryKey;column:subroot_offset"`
	Root            string `gorm:"primaryKey;column:root"`
	AddMessageOK    *bool
	AddMessageIndex *int64
	Subroot         string `gorm:"not null"`
	SubrootSize     int64

	PDPPieceRefID *int64       // references pdp_piecerefs(id)
	PDPPieceRef   *PDPPieceRef `gorm:"foreignKey:PDPPieceRefID;references:ID;constraint:OnDelete:SET NULL"`
}

func (PDPProofsetRootAdd) TableName() string {
	return "pdp_proofset_root_adds"
}

// pdp_provider_registrations
type PDPProviderRegistration struct {
	RegisterMessageHash string           `gorm:"primaryKey"` // references message_waits_eth(signed_tx_hash)
	MessageWait         *MessageWaitsEth `gorm:"foreignKey:RegisterMessageHash;references:SignedTxHash;constraint:OnDelete:CASCADE"`

	Ok                 *bool  // NULL / TRUE / FALSE
	ProviderRegistered bool   `gorm:"default:false;not null"`
	ProviderID         *int64 // The provider ID assigned after registration
	Service            string `gorm:"not null"` // references pdp_services(service_label)

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;not null"`
}

func (PDPProviderRegistration) TableName() string {
	return "pdp_provider_registrations"
}

// MessageSendsEth represents the message_sends_eth table.
type MessageSendsEth struct {
	FromAddress  string     `gorm:"not null;column:from_address"`
	ToAddress    string     `gorm:"not null;column:to_address"`
	SendReason   string     `gorm:"not null;column:send_reason"`
	SendTaskID   int        `gorm:"primaryKey;autoIncrement;column:send_task_id"`
	UnsignedTx   []byte     `gorm:"not null;column:unsigned_tx"`
	UnsignedHash string     `gorm:"not null;column:unsigned_hash"`
	Nonce        *int64     `gorm:"column:nonce"`
	SignedTx     []byte     `gorm:"column:signed_tx"`
	SignedHash   *string    `gorm:"column:signed_hash"`
	SendTime     *time.Time `gorm:"column:send_time"`
	SendSuccess  *bool      `gorm:"column:send_success"`
	SendError    *string    `gorm:"column:send_error"`
}

func (MessageSendsEth) TableName() string {
	return "message_sends_eth"
}

// MessageSendEthLock represents the message_send_eth_locks table.
type MessageSendEthLock struct {
	FromAddress string    `gorm:"primaryKey;column:from_address;not null"`
	TaskID      int64     `gorm:"not null;column:task_id"`
	ClaimedAt   time.Time `gorm:"not null;column:claimed_at"`
}

func (MessageSendEthLock) TableName() string {
	return "message_send_eth_locks"
}

// MessageWaitsEth represents the message_waits_eth table.
type MessageWaitsEth struct {
	WaiterMachineID      *int64         `gorm:"column:waiter_machine_id"`
	SignedTxHash         string         `gorm:"primaryKey;column:signed_tx_hash;not null"`
	ConfirmedBlockNumber *int64         `gorm:"column:confirmed_block_number"`
	ConfirmedTxHash      string         `gorm:"column:confirmed_tx_hash"`
	ConfirmedTxData      datatypes.JSON `gorm:"column:confirmed_tx_data"`
	TxStatus             string         `gorm:"column:tx_status"`
	TxReceipt            datatypes.JSON `gorm:"column:tx_receipt"`
	TxSuccess            *bool          `gorm:"column:tx_success"`
}

func (MessageWaitsEth) TableName() string {
	return "message_waits_eth"
}

func Ptr[T any](v T) *T {
	return &v
}

func AutoMigrateDB(ctx context.Context, db *gorm.DB) error {
	if err := db.
		WithContext(ctx).
		AutoMigrate(
			&Task{},
			&TaskHistory{},

			&ParkedPiece{},
			&ParkedPieceRef{},

			&PDPPieceUpload{},
			&PDPPieceRef{},
			&PDPProofSet{},
			&PDPProveTask{},
			&PDPProofsetCreate{},
			&PDPProofsetRoot{},
			&PDPProofsetRootAdd{},
			&PDPPieceMHToCommp{},
			&PDPProviderRegistration{},

			&MessageSendsEth{},
			&MessageSendEthLock{},
			&MessageWaitsEth{},
		); err != nil {
		return fmt.Errorf("failed to auto migrate database: %s", err)
	}
	if err := db.
		WithContext(ctx).
		Exec(Triggers).Error; err != nil {
		return fmt.Errorf("failed to install database triggers: %s", err)
	}
	return nil
}
