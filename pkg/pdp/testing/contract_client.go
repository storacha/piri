package testing

import (
	"go.uber.org/mock/gomock"

	"github.com/storacha/piri/pkg/pdp/contract/mocks"
)

func NewMockContractClient(ctrl *gomock.Controller) (*mocks.MockPDP, *mocks.MockPDPVerifier, *mocks.MockPDPProvingSchedule) {
	mockPDP := mocks.NewMockPDP(ctrl)
	mockVerifier := mocks.NewMockPDPVerifier(ctrl)
	mockSchedule := mocks.NewMockPDPProvingSchedule(ctrl)

	// Setup the PDP mock to return our mocked verifier and schedule
	mockPDP.EXPECT().NewPDPVerifier(gomock.Any(), gomock.Any()).Return(mockVerifier, nil).AnyTimes()
	mockPDP.EXPECT().NewIPDPProvingSchedule(gomock.Any(), gomock.Any()).Return(mockSchedule, nil).AnyTimes()

	return mockPDP, mockVerifier, mockSchedule
}
