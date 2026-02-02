package payment

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/storacha/piri/pkg/admin/httpapi"
	"github.com/storacha/piri/pkg/admin/httpapi/client"
)

// Styles for the TUI
var (
	docStyle = lipgloss.NewStyle().Padding(1, 2, 1, 2)

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(22)
	valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	// Confirmation view styles
	boxStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(1, 2)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

// View states
type viewState int

const (
	viewMain viewState = iota
	viewConfirmSettle
	viewSettling       // Sending transaction
	viewWaitingConfirm // Waiting for on-chain confirmation
	viewSettled
	// Withdraw states
	viewEnterWithdrawAddress   // Address selection
	viewConfirmWithdraw        // Confirmation screen
	viewWithdrawing            // Sending transaction
	viewWaitingWithdrawConfirm // Waiting for on-chain confirmation
	viewWithdrawn              // Success
)

// Spinner frames for animation
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Message types for async operations
type statusRefreshMsg struct {
	accountInfo *httpapi.GetAccountInfoResponse
	err         error
}

type estimateMsg struct {
	estimate *httpapi.EstimateSettlementResponse
	err      error
}

type settleMsg struct {
	txHash string
	status string
	err    error
}

type animationTickMsg struct{}

type settlementStatusMsg struct {
	status *httpapi.SettlementStatusResponse
	err    error
}

// Withdraw message types
type withdrawEstimateMsg struct {
	estimate *httpapi.EstimateWithdrawResponse
	err      error
}

type withdrawMsg struct {
	txHash string
	status string
	err    error
}

type withdrawalStatusMsg struct {
	status *httpapi.WithdrawalStatusResponse
	err    error
}

// statusModel is the Bubbletea model for the payment status TUI
type statusModel struct {
	accountInfo *httpapi.GetAccountInfoResponse
	table       table.Model

	// For refresh
	apiClient    *client.Client
	lastRefresh  time.Time
	refreshError error

	// For settlement
	viewState      viewState
	selectedRail   *httpapi.RailView
	settleEstimate *httpapi.EstimateSettlementResponse
	settleError    error
	settleTxHash   string
	animationFrame int

	// For withdrawal
	withdrawRecipient string
	withdrawEstimate  *httpapi.EstimateWithdrawResponse
	withdrawTxHash    string
	withdrawError     error
}

func newStatusModel(accountInfo *httpapi.GetAccountInfoResponse, apiClient *client.Client) statusModel {
	m := statusModel{
		apiClient:   apiClient,
		lastRefresh: time.Now(),
		viewState:   viewMain,
	}
	m.updateFromAccountInfo(accountInfo)
	return m
}

func (m *statusModel) updateFromAccountInfo(accountInfo *httpapi.GetAccountInfoResponse) {
	m.accountInfo = accountInfo
	m.table = buildRailsTable(accountInfo.Rails)
}

func (m statusModel) Init() tea.Cmd {
	return nil
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.viewState {
		case viewMain:
			return m.handleMainKeys(msg)
		case viewConfirmSettle:
			return m.handleConfirmKeys(msg)
		case viewSettling, viewWaitingConfirm:
			// No key handling while settling/waiting
			return m, nil
		case viewSettled:
			return m.handleSettledKeys(msg)
		case viewEnterWithdrawAddress:
			return m.handleEnterAddressKeys(msg)
		case viewConfirmWithdraw:
			return m.handleConfirmWithdrawKeys(msg)
		case viewWithdrawing, viewWaitingWithdrawConfirm:
			// No key handling while withdrawing/waiting
			return m, nil
		case viewWithdrawn:
			return m.handleWithdrawnKeys(msg)
		}

	case statusRefreshMsg:
		if msg.err != nil {
			m.refreshError = msg.err
			return m, nil
		}
		m.refreshError = nil
		m.lastRefresh = time.Now()
		m.updateFromAccountInfo(msg.accountInfo)
		return m, nil

	case estimateMsg:
		if msg.err != nil {
			m.settleError = msg.err
			m.viewState = viewMain
			return m, nil
		}
		m.settleEstimate = msg.estimate
		m.viewState = viewConfirmSettle
		return m, nil

	case settleMsg:
		if msg.err != nil {
			m.settleError = msg.err
			m.viewState = viewMain
			return m, nil
		}
		m.settleTxHash = msg.txHash
		m.viewState = viewWaitingConfirm
		// Start polling for confirmation
		return m, tea.Batch(
			m.tickAnimation(),
			m.pollSettlementStatus(),
		)

	case animationTickMsg:
		m.animationFrame = (m.animationFrame + 1) % len(spinnerFrames)
		// Continue animation only while settling/waiting/withdrawing
		if m.viewState == viewSettling || m.viewState == viewWaitingConfirm ||
			m.viewState == viewWithdrawing || m.viewState == viewWaitingWithdrawConfirm {
			return m, m.tickAnimation()
		}
		return m, nil

	case settlementStatusMsg:
		if msg.err != nil {
			// Continue polling on error
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return m.pollSettlementStatus()()
			})
		}
		if msg.status.Status == "confirmed" {
			m.viewState = viewSettled
			return m, nil
		}
		// Still pending, continue polling
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return m.pollSettlementStatus()()
		})

	case withdrawEstimateMsg:
		if msg.err != nil {
			m.withdrawError = msg.err
			m.viewState = viewMain
			return m, nil
		}
		m.withdrawEstimate = msg.estimate
		m.viewState = viewConfirmWithdraw
		return m, nil

	case withdrawMsg:
		if msg.err != nil {
			m.withdrawError = msg.err
			m.viewState = viewMain
			return m, nil
		}
		m.withdrawTxHash = msg.txHash
		m.viewState = viewWaitingWithdrawConfirm
		// Start polling for confirmation
		return m, tea.Batch(
			m.tickAnimation(),
			m.pollWithdrawalStatus(),
		)

	case withdrawalStatusMsg:
		if msg.err != nil {
			// Continue polling on error
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return m.pollWithdrawalStatus()()
			})
		}
		if msg.status.Status == "confirmed" {
			m.viewState = viewWithdrawn
			return m, nil
		}
		// Still pending, continue polling
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return m.pollWithdrawalStatus()()
		})
	}

	// Update the table (for scrolling) - only in main view
	if m.viewState == viewMain {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m statusModel) handleMainKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "r":
		return m, m.fetchStatus()
	case "shift+enter", "S":
		// Initiate settlement for selected rail
		if len(m.accountInfo.Rails) > 0 {
			selectedIdx := m.table.Cursor()
			if selectedIdx >= 0 && selectedIdx < len(m.accountInfo.Rails) {
				m.selectedRail = &m.accountInfo.Rails[selectedIdx]
				m.settleError = nil
				m.settleEstimate = nil
				return m, m.fetchEstimate()
			}
		}
	case "W":
		// Initiate withdrawal
		m.withdrawError = nil
		m.withdrawEstimate = nil
		m.withdrawRecipient = "" // Will default to owner address
		m.viewState = viewEnterWithdrawAddress
		return m, nil
	}

	// Let table handle navigation keys
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m statusModel) handleConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "enter", "y":
		// Confirm settlement
		m.viewState = viewSettling
		m.animationFrame = 0
		return m, tea.Batch(
			m.submitSettle(),
			m.tickAnimation(),
		)
	case "esc", "n":
		// Cancel - return to main view
		m.viewState = viewMain
		m.selectedRail = nil
		m.settleEstimate = nil
		m.settleError = nil
		return m, nil
	}
	return m, nil
}

func (m statusModel) handleSettledKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "enter", "esc":
		// Return to main view and refresh
		m.viewState = viewMain
		m.selectedRail = nil
		m.settleEstimate = nil
		m.settleTxHash = ""
		m.animationFrame = 0
		return m, m.fetchStatus()
	}
	return m, nil
}

func (m statusModel) handleEnterAddressKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "enter":
		// Use current address (empty means default owner) and fetch estimate
		m.viewState = viewConfirmWithdraw
		return m, m.fetchWithdrawEstimate()
	case "esc":
		// Cancel - return to main view
		m.viewState = viewMain
		m.withdrawRecipient = ""
		m.withdrawError = nil
		return m, nil
	case "backspace":
		if len(m.withdrawRecipient) > 0 {
			m.withdrawRecipient = m.withdrawRecipient[:len(m.withdrawRecipient)-1]
		}
		return m, nil
	default:
		// Handle character input for address
		key := msg.String()
		if len(key) == 1 {
			m.withdrawRecipient += key
		}
		return m, nil
	}
}

func (m statusModel) handleConfirmWithdrawKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "enter", "y":
		// Confirm withdrawal
		m.viewState = viewWithdrawing
		m.animationFrame = 0
		return m, tea.Batch(
			m.submitWithdraw(),
			m.tickAnimation(),
		)
	case "esc", "n":
		// Cancel - return to main view
		m.viewState = viewMain
		m.withdrawRecipient = ""
		m.withdrawEstimate = nil
		m.withdrawError = nil
		return m, nil
	}
	return m, nil
}

func (m statusModel) handleWithdrawnKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "enter", "esc":
		// Return to main view and refresh
		m.viewState = viewMain
		m.withdrawRecipient = ""
		m.withdrawEstimate = nil
		m.withdrawTxHash = ""
		m.animationFrame = 0
		return m, m.fetchStatus()
	}
	return m, nil
}

func (m statusModel) fetchStatus() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		accountInfo, err := m.apiClient.GetAccountInfo(ctx)
		return statusRefreshMsg{accountInfo: accountInfo, err: err}
	}
}

func (m statusModel) fetchEstimate() tea.Cmd {
	railID := m.selectedRail.RailID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		estimate, err := m.apiClient.EstimateSettlement(ctx, railID)
		return estimateMsg{estimate: estimate, err: err}
	}
}

func (m statusModel) submitSettle() tea.Cmd {
	railID := m.selectedRail.RailID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := m.apiClient.SettleRail(ctx, railID)
		if err != nil {
			return settleMsg{err: err}
		}
		return settleMsg{txHash: result.TxHash, status: result.Status}
	}
}

func (m statusModel) tickAnimation() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return animationTickMsg{}
	})
}

func (m statusModel) pollSettlementStatus() tea.Cmd {
	railID := m.selectedRail.RailID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := m.apiClient.GetSettlementStatus(ctx, railID)
		return settlementStatusMsg{status: status, err: err}
	}
}

func (m statusModel) fetchWithdrawEstimate() tea.Cmd {
	recipient := m.withdrawRecipient
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		estimate, err := m.apiClient.EstimateWithdraw(ctx, recipient, "")
		return withdrawEstimateMsg{estimate: estimate, err: err}
	}
}

func (m statusModel) submitWithdraw() tea.Cmd {
	recipient := m.withdrawRecipient
	amount := ""
	if m.withdrawEstimate != nil {
		amount = m.withdrawEstimate.WithdrawAmount
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := m.apiClient.Withdraw(ctx, recipient, amount)
		if err != nil {
			return withdrawMsg{err: err}
		}
		return withdrawMsg{txHash: result.TxHash, status: result.Status}
	}
}

func (m statusModel) pollWithdrawalStatus() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := m.apiClient.GetWithdrawalStatus(ctx)
		return withdrawalStatusMsg{status: status, err: err}
	}
}

func (m statusModel) View() string {
	switch m.viewState {
	case viewConfirmSettle:
		return m.renderConfirmSettle()
	case viewSettling, viewWaitingConfirm:
		return m.renderSettling()
	case viewSettled:
		return m.renderSettled()
	case viewEnterWithdrawAddress:
		return m.renderEnterWithdrawAddress()
	case viewConfirmWithdraw:
		return m.renderConfirmWithdraw()
	case viewWithdrawing, viewWaitingWithdrawConfirm:
		return m.renderWithdrawing()
	case viewWithdrawn:
		return m.renderWithdrawn()
	default:
		return m.renderMain()
	}
}

func (m statusModel) renderMain() string {
	doc := strings.Builder{}

	// Render overview at top
	doc.WriteString(m.renderOverview())
	doc.WriteString("\n")

	// Render rails table
	doc.WriteString(titleStyle.Render("RAILS"))
	doc.WriteString("\n")
	if len(m.accountInfo.Rails) > 0 {
		doc.WriteString(m.table.View())
	} else {
		doc.WriteString(helpStyle.Render("No payment rails found"))
	}
	doc.WriteString("\n\n")

	// Show errors
	if m.settleError != nil {
		doc.WriteString(errorStyle.Render("Settlement error: " + m.settleError.Error()))
		doc.WriteString("\n")
	}
	if m.withdrawError != nil {
		doc.WriteString(errorStyle.Render("Withdrawal error: " + m.withdrawError.Error()))
		doc.WriteString("\n")
	}

	// Show refresh status
	if m.refreshError != nil {
		doc.WriteString(errorStyle.Render("Refresh error: " + m.refreshError.Error()))
		doc.WriteString("\n")
	} else if !m.lastRefresh.IsZero() {
		ago := time.Since(m.lastRefresh).Round(time.Second)
		doc.WriteString(helpStyle.Render("Last refresh: " + ago.String() + " ago"))
		doc.WriteString("\n")
	}

	doc.WriteString(helpStyle.Render("↑ ↓ scroll │ r refresh │ S settle selected │ W withdraw │ q quit"))

	return docStyle.Render(doc.String())
}

func (m statusModel) renderConfirmSettle() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("CONFIRM SETTLEMENT"))
	b.WriteString("\n\n")

	if m.settleEstimate == nil {
		b.WriteString(helpStyle.Render("Loading estimate..."))
		return docStyle.Render(b.String())
	}

	est := m.settleEstimate

	// Rail info
	b.WriteString(labelStyle.Render("Rail ID:"))
	b.WriteString(valueStyle.Render(est.RailID))
	b.WriteString("\n")

	if est.DataSetID != "" && est.DataSetID != "0" {
		b.WriteString(labelStyle.Render("Dataset ID:"))
		b.WriteString(valueStyle.Render(est.DataSetID))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Settlement breakdown
	b.WriteString(titleStyle.Render("SETTLEMENT BREAKDOWN"))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Gross Settleable:"))
	b.WriteString(valueStyle.Render(formatTokenAmount(est.GrossSettleableAmount)))
	b.WriteString("\n")

	// Show proof penalty if there is a reduction
	proofPct := parseOrZero(est.ProofReductionPct)
	if proofPct.Sign() > 0 {
		// Calculate the penalty amount
		gross := parseOrZero(est.GrossSettleableAmount)
		net := parseOrZero(est.NetSettleableAmount)
		penalty := new(big.Int).Sub(gross, net)
		b.WriteString(labelStyle.Render("Proof Penalty:"))
		b.WriteString(errorStyle.Render(fmt.Sprintf("-%s (%s%% missed)", formatTokenAmountBigInt(penalty), est.ProofReductionPct)))
		b.WriteString("\n")
	}

	b.WriteString(labelStyle.Render("Net Settleable:"))
	b.WriteString(valueStyle.Render(formatTokenAmount(est.NetSettleableAmount)))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Network Fee (0.5%):"))
	b.WriteString(warningStyle.Render("-" + formatTokenAmount(est.NetworkFee)))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Final Amount:"))
	b.WriteString(successStyle.Render(formatTokenAmount(est.NetAmount)))
	b.WriteString("\n\n")

	// Gas estimate
	b.WriteString(titleStyle.Render("GAS ESTIMATE"))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Gas Limit:"))
	b.WriteString(valueStyle.Render(formatBigIntWithCommas(parseOrZero(est.GasLimit))))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Gas Price:"))
	b.WriteString(valueStyle.Render(formatGasPrice(est.GasPrice)))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Gas Cost (FIL):"))
	b.WriteString(warningStyle.Render(formatFIL(est.GasCost)))
	b.WriteString("\n\n")

	// Epoch info
	b.WriteString(labelStyle.Render("Settle until epoch:"))
	b.WriteString(valueStyle.Render(formatEpoch(est.UntilEpoch)))
	b.WriteString("\n\n")

	// Action prompt
	b.WriteString(boxStyle.Render("Press [Enter] to confirm or [Esc] to cancel"))

	return docStyle.Render(b.String())
}

func (m statusModel) renderSettling() string {
	var b strings.Builder
	spinner := spinnerFrames[m.animationFrame]

	// Title based on state
	title := "SETTLING RAIL"
	if m.viewState == viewWaitingConfirm {
		title = "WAITING FOR CONFIRMATION"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Show the settlement details (same as confirmation screen)
	if m.settleEstimate != nil {
		est := m.settleEstimate

		b.WriteString(labelStyle.Render("Rail ID:"))
		b.WriteString(valueStyle.Render(est.RailID))
		b.WriteString("\n")

		if est.DataSetID != "" && est.DataSetID != "0" {
			b.WriteString(labelStyle.Render("Dataset ID:"))
			b.WriteString(valueStyle.Render(est.DataSetID))
			b.WriteString("\n")
		}
		b.WriteString("\n")

		// Settlement amounts
		b.WriteString(labelStyle.Render("Net Amount:"))
		b.WriteString(successStyle.Render(formatTokenAmount(est.NetAmount)))
		b.WriteString("\n\n")
	}

	// Status message with spinner
	if m.viewState == viewSettling {
		b.WriteString(warningStyle.Render(spinner + " Sending transaction..."))
	} else {
		b.WriteString(warningStyle.Render(spinner + " Pending confirmation..."))
		if m.settleTxHash != "" {
			b.WriteString("\n\n")
			b.WriteString(labelStyle.Render("Transaction Hash:"))
			b.WriteString("\n")
			b.WriteString(valueStyle.Render(m.settleTxHash))
		}
	}
	b.WriteString("\n\n")

	b.WriteString(helpStyle.Render("Please wait, this may take a few minutes."))

	return docStyle.Render(b.String())
}

func (m statusModel) renderSettled() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("SETTLEMENT CONFIRMED"))
	b.WriteString("\n\n")

	b.WriteString(successStyle.Render("Transaction confirmed on chain!"))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Transaction Hash:"))
	b.WriteString("\n")
	b.WriteString(valueStyle.Render(m.settleTxHash))
	b.WriteString("\n\n")

	b.WriteString(helpStyle.Render("The settlement has been confirmed."))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Your balance will be updated shortly."))
	b.WriteString("\n\n")

	b.WriteString(boxStyle.Render("Press [Enter] to return to main view"))

	return docStyle.Render(b.String())
}

func (m statusModel) renderEnterWithdrawAddress() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("WITHDRAW FUNDS"))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Available to Withdraw:"))
	b.WriteString(successStyle.Render(formatTokenAmount(m.accountInfo.AvailableToWithdraw)))
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render("RECIPIENT ADDRESS"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter recipient address or press Enter to use default (owner address)"))
	b.WriteString("\n\n")

	// Show address input field
	addressDisplay := m.withdrawRecipient
	if addressDisplay == "" {
		addressDisplay = helpStyle.Render("(default: owner address)")
	}
	b.WriteString(labelStyle.Render("Recipient:"))
	b.WriteString(valueStyle.Render(addressDisplay))
	b.WriteString("\n\n")

	b.WriteString(boxStyle.Render("Press [Enter] to continue or [Esc] to cancel"))

	return docStyle.Render(b.String())
}

func (m statusModel) renderConfirmWithdraw() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("CONFIRM WITHDRAWAL"))
	b.WriteString("\n\n")

	if m.withdrawEstimate == nil {
		b.WriteString(helpStyle.Render("Loading estimate..."))
		return docStyle.Render(b.String())
	}

	est := m.withdrawEstimate

	// Withdrawal info
	b.WriteString(labelStyle.Render("Recipient:"))
	b.WriteString(valueStyle.Render(est.Recipient))
	b.WriteString("\n\n")

	// Withdrawal breakdown
	b.WriteString(titleStyle.Render("WITHDRAWAL BREAKDOWN"))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Available to Withdraw:"))
	b.WriteString(valueStyle.Render(formatTokenAmount(est.AvailableToWithdraw)))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Withdraw Amount:"))
	b.WriteString(successStyle.Render(formatTokenAmount(est.WithdrawAmount)))
	b.WriteString("\n\n")

	// Gas estimate
	b.WriteString(titleStyle.Render("GAS ESTIMATE"))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Gas Limit:"))
	b.WriteString(valueStyle.Render(formatBigIntWithCommas(parseOrZero(est.GasLimit))))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Gas Price:"))
	b.WriteString(valueStyle.Render(formatGasPrice(est.GasPrice)))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Gas Cost (FIL):"))
	b.WriteString(warningStyle.Render(formatFIL(est.GasCost)))
	b.WriteString("\n\n")

	// Action prompt
	b.WriteString(boxStyle.Render("Press [Enter] to confirm or [Esc] to cancel"))

	return docStyle.Render(b.String())
}

func (m statusModel) renderWithdrawing() string {
	var b strings.Builder
	spinner := spinnerFrames[m.animationFrame]

	// Title based on state
	title := "WITHDRAWING"
	if m.viewState == viewWaitingWithdrawConfirm {
		title = "WAITING FOR CONFIRMATION"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Show the withdrawal details
	if m.withdrawEstimate != nil {
		est := m.withdrawEstimate

		b.WriteString(labelStyle.Render("Recipient:"))
		b.WriteString(valueStyle.Render(est.Recipient))
		b.WriteString("\n")

		b.WriteString(labelStyle.Render("Withdraw Amount:"))
		b.WriteString(successStyle.Render(formatTokenAmount(est.WithdrawAmount)))
		b.WriteString("\n\n")
	}

	// Status message with spinner
	if m.viewState == viewWithdrawing {
		b.WriteString(warningStyle.Render(spinner + " Sending transaction..."))
	} else {
		b.WriteString(warningStyle.Render(spinner + " Pending confirmation..."))
		if m.withdrawTxHash != "" {
			b.WriteString("\n\n")
			b.WriteString(labelStyle.Render("Transaction Hash:"))
			b.WriteString("\n")
			b.WriteString(valueStyle.Render(m.withdrawTxHash))
		}
	}
	b.WriteString("\n\n")

	b.WriteString(helpStyle.Render("Please wait, this may take a few minutes."))

	return docStyle.Render(b.String())
}

func (m statusModel) renderWithdrawn() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("WITHDRAWAL CONFIRMED"))
	b.WriteString("\n\n")

	b.WriteString(successStyle.Render("Transaction confirmed on chain!"))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Transaction Hash:"))
	b.WriteString("\n")
	b.WriteString(valueStyle.Render(m.withdrawTxHash))
	b.WriteString("\n\n")

	b.WriteString(helpStyle.Render("The withdrawal has been confirmed."))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Funds have been sent to the recipient."))
	b.WriteString("\n\n")

	b.WriteString(boxStyle.Render("Press [Enter] to return to main view"))

	return docStyle.Render(b.String())
}

func (m statusModel) renderOverview() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("PAYMENT STATUS OVERVIEW"))
	b.WriteString("\n\n")

	// Account summary
	b.WriteString(titleStyle.Render("ACCOUNT SUMMARY"))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Current Epoch:"))
	b.WriteString(valueStyle.Render(formatEpoch(m.accountInfo.CurrentEpoch)))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Balance (withdrawable):"))
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(
		formatTokenAmount(m.accountInfo.Funds)))
	b.WriteString("\n\n")

	// Aggregate stats
	totalGross, totalNet, totalUnsettled := m.calculateAggregates()

	b.WriteString(titleStyle.Render("AGGREGATE STATS"))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Total Rails/Datasets:"))
	b.WriteString(valueStyle.Render(formatBigIntWithCommas(big.NewInt(int64(len(m.accountInfo.Rails))))))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Total Gross Settleable:"))
	b.WriteString(valueStyle.Render(formatTokenAmountBigInt(totalGross)))
	b.WriteString("\n")

	// Show net settleable with percentage of gross
	netPctStr := ""
	if totalGross.Sign() > 0 && totalNet.Cmp(totalGross) != 0 {
		pct := new(big.Int).Mul(totalNet, big.NewInt(100))
		pct = pct.Div(pct, totalGross)
		netPctStr = fmt.Sprintf(" (%s%% of gross)", pct.String())
	}
	b.WriteString(labelStyle.Render("Total Net Settleable:"))
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(
		formatTokenAmountBigInt(totalNet) + netPctStr))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Total Unsettled:"))
	b.WriteString(valueStyle.Render(formatTokenAmountBigInt(totalUnsettled)))
	b.WriteString("\n")

	return b.String()
}

func (m statusModel) calculateAggregates() (totalGross, totalNet, totalUnsettled *big.Int) {
	totalGross = big.NewInt(0)
	totalNet = big.NewInt(0)
	totalUnsettled = big.NewInt(0)

	for _, rail := range m.accountInfo.Rails {
		if amt, ok := new(big.Int).SetString(rail.SettleableAmount, 10); ok {
			totalGross.Add(totalGross, amt)
		}
		if amt, ok := new(big.Int).SetString(rail.NetSettleableAmount, 10); ok {
			totalNet.Add(totalNet, amt)
		}
		if amt, ok := new(big.Int).SetString(rail.UnsettledAmount, 10); ok {
			totalUnsettled.Add(totalUnsettled, amt)
		}
	}
	return
}

func formatTokenAmountBigInt(wei *big.Int) string {
	if wei == nil || wei.Sign() == 0 {
		return "$0.00"
	}
	return formatTokenAmount(wei.String())
}

func buildRailsTable(rails []httpapi.RailView) table.Model {
	columns := []table.Column{
		{Title: "Rail", Width: 6},
		{Title: "DS ID", Width: 6},
		{Title: "From", Width: 12},
		{Title: "Rate/ep", Width: 10},
		{Title: "Settled To", Width: 10},
		{Title: "Gross", Width: 9},
		{Title: "Net", Width: 9},
		{Title: "Unsettled", Width: 9},
		{Title: "Status", Width: 8},
	}

	var rows []table.Row
	for _, rail := range rails {
		dsID := rail.DataSetID
		if dsID == "" || dsID == "0" {
			dsID = "-"
		}

		rows = append(rows, table.Row{
			rail.RailID,
			dsID,
			formatAddress(rail.From),
			formatRate(rail.PaymentRate),
			formatEpoch(rail.SettledUpTo),
			formatTokenCompact(rail.SettleableAmount),
			formatTokenCompact(rail.NetSettleableAmount),
			formatTokenCompact(rail.UnsettledAmount),
			formatStatus(rail.IsTerminated),
		})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(min(len(rows)+1, 15)),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return t
}

// parseOrZero parses a string to big.Int, returning zero on failure
func parseOrZero(s string) *big.Int {
	if n, ok := new(big.Int).SetString(s, 10); ok {
		return n
	}
	return big.NewInt(0)
}

// formatGasPrice formats gas price in attoFIL (Filecoin's base unit)
func formatGasPrice(attoStr string) string {
	atto := parseOrZero(attoStr)
	if atto.Sign() == 0 {
		return "0 attoFIL/gas"
	}
	return formatBigIntWithCommas(atto) + " attoFIL/gas"
}

// formatFIL formats a value in attoFIL (10^-18) to FIL with proper decimal places
func formatFIL(attoStr string) string {
	atto := parseOrZero(attoStr)
	if atto.Sign() == 0 {
		return "0 FIL"
	}
	// Convert attoFIL to FIL (1 FIL = 10^18 attoFIL)
	fil := new(big.Float).Quo(
		new(big.Float).SetInt(atto),
		new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
	)
	f, _ := fil.Float64()

	if f >= 1000 {
		return fmt.Sprintf("%.2f FIL", f)
	} else if f >= 1 {
		return fmt.Sprintf("%.4f FIL", f)
	} else if f >= 0.0001 {
		return fmt.Sprintf("%.6f FIL", f)
	} else if f >= 0.0000001 {
		return fmt.Sprintf("%.10f FIL", f)
	}
	return fmt.Sprintf("%.18f FIL", f)
}

