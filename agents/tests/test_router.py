from app.router import route_intent


def test_route_intent_maps_refund_query_to_refund():
    state = {"messages": [{"role": "user", "content": "订单 93001 可以退款吗"}]}

    assert route_intent(state) == "refund"
