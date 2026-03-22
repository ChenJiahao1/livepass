from app.router import route_intent


def test_route_intent_maps_refund_query_to_refund():
    state = {"messages": [{"role": "user", "content": "订单 93001 可以退款吗"}]}

    assert route_intent(state) == "refund"


def test_route_intent_maps_knowledge_query_to_knowledge():
    state = {"messages": [{"role": "user", "content": "周杰伦是谁"}]}

    assert route_intent(state) == "knowledge"


def test_route_intent_maps_handoff_query_to_handoff():
    state = {"messages": [{"role": "user", "content": "我要转人工"}]}

    assert route_intent(state) == "handoff"


def test_route_intent_maps_activity_query_to_activity():
    state = {"messages": [{"role": "user", "content": "上海有什么演出"}]}

    assert route_intent(state) == "activity"


def test_route_intent_maps_ord_and_payment_query_to_order():
    state = {"messages": [{"role": "user", "content": "ORD-1001 支付了吗"}]}

    assert route_intent(state) == "order"


def test_route_intent_falls_back_to_handoff_when_state_requires_it():
    state = {
        "messages": [{"role": "user", "content": "还是不太明白"}],
        "need_handoff": True,
    }

    assert route_intent(state) == "handoff"
