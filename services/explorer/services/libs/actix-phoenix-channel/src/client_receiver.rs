use crate::{event::ChannelEvent, subscription_registry::SubscriptionRegistry};
use async_broadcast::Receiver as BroadcastReceiver;
use async_channel::Receiver;
use futures_lite::{Stream, StreamExt, stream::Race};
use std::{
    pin::Pin,
    task::{Context, Poll},
};

#[derive(Debug)]
pub struct ClientReceiver {
    race: Pin<Box<Race<SubscribedBroadcasts, Receiver<ChannelEvent>>>>,
}

#[derive(Debug)]
struct SubscribedBroadcasts {
    receiver: BroadcastReceiver<ChannelEvent>,
    subscriptions: SubscriptionRegistry,
}

impl Stream for SubscribedBroadcasts {
    type Item = ChannelEvent;

    fn poll_next(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Option<Self::Item>> {
        loop {
            match Pin::new(&mut self.receiver).poll_next(cx) {
                Poll::Ready(Some(event)) if !self.subscriptions.subscribes(&event) => continue,
                other => return other,
            }
        }
    }
}

impl ClientReceiver {
    pub fn new(
        individual: Receiver<ChannelEvent>,
        broadcast: BroadcastReceiver<ChannelEvent>,
        subscriptions: SubscriptionRegistry,
    ) -> Self {
        let broadcast = SubscribedBroadcasts {
            receiver: broadcast,
            subscriptions,
        };
        Self {
            race: Box::pin(broadcast.race(individual)),
        }
    }
}

impl Stream for ClientReceiver {
    type Item = String;

    fn poll_next(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Option<Self::Item>> {
        loop {
            match self.race.poll_next(cx) {
                Poll::Ready(Some(event)) => {
                    if let Ok(text) = event.serialize() {
                        break Poll::Ready(Some(text));
                    }
                }
                Poll::Pending => break Poll::Pending,
                Poll::Ready(None) => break Poll::Ready(None),
            }
        }
    }
}
