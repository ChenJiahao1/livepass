package logic

import (
	"livepass/services/user-api/internal/types"
	"livepass/services/user-rpc/userrpc"
)

func mapBoolResp(resp *userrpc.BoolResp) *types.BoolResp {
	if resp == nil {
		return &types.BoolResp{}
	}

	return &types.BoolResp{Success: resp.Success}
}

func mapUserVo(resp *userrpc.UserInfo) *types.UserVo {
	if resp == nil {
		return &types.UserVo{}
	}

	return &types.UserVo{
		ID:                      resp.Id,
		Name:                    resp.Name,
		RelName:                 resp.RelName,
		Gender:                  resp.Gender,
		Mobile:                  resp.Mobile,
		EmailStatus:             resp.EmailStatus,
		Email:                   resp.Email,
		RelAuthenticationStatus: resp.RelAuthenticationStatus,
		IdNumber:                resp.IdNumber,
		Address:                 resp.Address,
	}
}

func mapTicketUserVo(resp *userrpc.TicketUserInfo) types.TicketUserVo {
	if resp == nil {
		return types.TicketUserVo{}
	}

	return types.TicketUserVo{
		ID:       resp.Id,
		UserID:   resp.UserId,
		RelName:  resp.RelName,
		IdType:   resp.IdType,
		IdNumber: resp.IdNumber,
	}
}

func mapTicketUserVoList(list []*userrpc.TicketUserInfo) []types.TicketUserVo {
	if len(list) == 0 {
		return nil
	}

	resp := make([]types.TicketUserVo, 0, len(list))
	for _, item := range list {
		resp = append(resp, mapTicketUserVo(item))
	}

	return resp
}
