#!/usr/bin/env bash
set -euo pipefail

agent_case_activity() {
  printf '%s\n' "最近有什么演出"
}

agent_case_order() {
  printf '%s\n' "帮我查一下订单"
}

agent_case_refund_preview() {
  printf '%s\n' "订单 43509738860707840 可以退款吗"
}

agent_case_refund_submit() {
  printf '%s\n' "帮我退款订单 43509738860707840"
}

agent_case_handoff() {
  printf '%s\n' "我要人工客服"
}
