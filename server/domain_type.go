package server

import (
	"net/http"

	"github.com/Layr-Labs/eigenda-proxy/common"
)

func ReadDomainFilter(r *http.Request) (common.DomainType, error) {
	query := r.URL.Query()
	key := query.Get(DomainFilterKey)
	if key == "" { // default
		return common.BinaryDomain, nil
	}
	dt := common.StrToDomainType(key)
	if dt == common.UnknownDomain {
		return common.UnknownDomain, common.ErrInvalidDomainType
	}

	return dt, nil
}
