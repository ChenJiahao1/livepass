package logic

import (
	"database/sql"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/pay-rpc/internal/model"
	"damai-go/services/pay-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	payStatusCreated int64 = 1
	payStatusPaid    int64 = 2

	payDateTimeLayout = "2006-01-02 15:04:05"
)

var nowFunc = time.Now

func validateMockPayReq(in *pb.MockPayReq) error {
	if in.GetOrderNumber() <= 0 || in.GetUserId() <= 0 || in.GetSubject() == "" || in.GetAmount() <= 0 {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func validateGetPayBillReq(in *pb.GetPayBillReq) error {
	if in.GetOrderNumber() <= 0 {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func normalizePayChannel(channel string) string {
	if channel == "" {
		return "mock"
	}

	return channel
}

func newNullTime(value time.Time) sql.NullTime {
	return sql.NullTime{
		Time:  value,
		Valid: true,
	}
}

func formatPayNullTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}

	return value.Time.Format(payDateTimeLayout)
}

func mapMockPayResp(payBill *model.DPayBill) *pb.MockPayResp {
	if payBill == nil {
		return &pb.MockPayResp{}
	}

	return &pb.MockPayResp{
		PayBillNo: payBill.PayBillNo,
		PayStatus: payBill.PayStatus,
		PayTime:   formatPayNullTime(payBill.PayTime),
	}
}

func mapGetPayBillResp(payBill *model.DPayBill) *pb.GetPayBillResp {
	if payBill == nil {
		return &pb.GetPayBillResp{}
	}

	return &pb.GetPayBillResp{
		PayBillNo:   payBill.PayBillNo,
		OrderNumber: payBill.OrderNumber,
		UserId:      payBill.UserId,
		Subject:     payBill.Subject,
		Channel:     payBill.Channel,
		Amount:      int64(payBill.OrderAmount),
		PayStatus:   payBill.PayStatus,
		PayTime:     formatPayNullTime(payBill.PayTime),
	}
}

func mapPayError(err error) error {
	switch {
	case err == nil:
		return nil
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, xerr.ErrInvalidParam):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, xerr.ErrPayBillNotFound), errors.Is(err, model.ErrNotFound):
		return status.Error(codes.NotFound, xerr.ErrPayBillNotFound.Error())
	default:
		return err
	}
}
