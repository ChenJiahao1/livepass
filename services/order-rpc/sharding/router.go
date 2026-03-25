package sharding

type Router interface {
	RouteByUserID(userID int64) (Route, error)
	RouteByOrderNumber(orderNumber int64) (Route, error)
}

type StaticRouter struct {
	routeMap *RouteMap
}

func NewStaticRouter(routeMap *RouteMap) *StaticRouter {
	return &StaticRouter{routeMap: routeMap}
}

func (r *StaticRouter) RouteByUserID(userID int64) (Route, error) {
	return r.routeMap.RouteByLogicSlot(LogicSlotByUserID(userID))
}

func (r *StaticRouter) RouteByOrderNumber(orderNumber int64) (Route, error) {
	logicSlot, err := LogicSlotByOrderNumber(orderNumber)
	if err != nil {
		return Route{}, err
	}

	return r.routeMap.RouteByLogicSlot(logicSlot)
}
