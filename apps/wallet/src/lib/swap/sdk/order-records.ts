import type { LimitOrder, OrderStatus, OrderType, StopLimitOrder, StopLossOrder } from './types';

type DecodedOrder<T> = {
  [K in keyof T]: K extends 'orderType' | 'status' ? bigint : T[K];
};

function orderEnum(value: bigint, field: string): OrderType | OrderStatus {
  // IPECOROrders declares five order types and five order statuses.
  if (typeof value !== 'bigint' || value < 0n || value > 4n) {
    throw new RangeError(`Invalid order ${field}: ${String(value)}`);
  }
  return Number(value) as OrderType | OrderStatus;
}

export function toLimitOrder(order: DecodedOrder<LimitOrder>): LimitOrder {
  return {
    id: order.id,
    user: order.user,
    stablecoin: order.stablecoin,
    token: order.token,
    amount: order.amount,
    targetPrice: order.targetPrice,
    orderType: orderEnum(order.orderType, 'orderType'),
    status: orderEnum(order.status, 'status'),
    createdAt: order.createdAt,
    expiresAt: order.expiresAt,
  };
}

export function toStopLossOrder(order: DecodedOrder<StopLossOrder>): StopLossOrder {
  return {
    id: order.id,
    user: order.user,
    stablecoin: order.stablecoin,
    token: order.token,
    amount: order.amount,
    triggerPrice: order.triggerPrice,
    status: orderEnum(order.status, 'status'),
    createdAt: order.createdAt,
    expiresAt: order.expiresAt,
  };
}

export function toStopLimitOrder(order: DecodedOrder<StopLimitOrder>): StopLimitOrder {
  return {
    id: order.id,
    user: order.user,
    stablecoin: order.stablecoin,
    token: order.token,
    amount: order.amount,
    stopPrice: order.stopPrice,
    limitPrice: order.limitPrice,
    orderType: orderEnum(order.orderType, 'orderType'),
    status: orderEnum(order.status, 'status'),
    createdAt: order.createdAt,
    expiresAt: order.expiresAt,
  };
}
