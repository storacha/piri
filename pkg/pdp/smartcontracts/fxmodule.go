package smartcontracts

import (
	"go.uber.org/fx"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	appconfig "github.com/storacha/piri/pkg/config/app"
)

var Module = fx.Module("smartcontracts",
	fx.Provide(
		ProvideRegistry,
		ProvideServiceView,
		ProvideServiceValidator,
		ProvideVerifierContract,
		ProvidePayment,
	),
)

func ProvideRegistry(cfg appconfig.PDPServiceConfig, client bind.ContractBackend) (Registry, error) {
	return NewRegistry(cfg.Contracts.ProviderRegistry, client)
}

func ProvideServiceView(cfg appconfig.PDPServiceConfig, client bind.ContractBackend) (Service, error) {
	return NewServiceView(cfg.Contracts.ServiceView, client)
}

func ProvideServiceValidator(cfg appconfig.PDPServiceConfig, client bind.ContractBackend) (ServiceValidator, error) {
	return NewServiceValidator(cfg.Contracts.Service, client)
}

func ProvideVerifierContract(cfg appconfig.PDPServiceConfig, client bind.ContractBackend) (Verifier, error) {
	return NewVerifierContract(cfg.Contracts.Verifier, client)
}

func ProvidePayment(cfg appconfig.PDPServiceConfig, client bind.ContractBackend) (Payment, error) {
	return NewPaymentContract(cfg.Contracts.Payments, client)
}
