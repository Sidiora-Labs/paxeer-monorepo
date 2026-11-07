import { Box, chakra, Flex, Text } from '@chakra-ui/react';
import { useQuery } from '@tanstack/react-query';
import React from 'react';

import type { InferenceDeal, InferenceRoom, InferenceReceipt } from 'lib/api/services/general/paxeerXInference';
import { inferenceQueryKey, isInferenceId, paxeerXInference } from 'lib/api/services/general/paxeerXInference';
import { Button } from 'toolkit/chakra/button';
import { Heading } from 'toolkit/chakra/heading';
import { Input } from 'toolkit/chakra/input';
import { TableBody, TableCell, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import { ScanTableCard } from 'ui/shared/scan';

const count = (value: string) => BigInt(value).toLocaleString('en-US');
const percent = (bps: number) => `${ bps / 100 }%`;

function Identifier({ value }: { value: string }) {
  return <Text as="span" fontFamily="mono" textStyle="xs" title={ value }>{ value.slice(0, 10) }…{ value.slice(-8) }</Text>;
}

export function InferenceRoomsTable({ rooms, onSelect }: { rooms: Array<InferenceRoom>; onSelect: (id: string) => void }) {
  const handleRoom = React.useCallback((event: React.MouseEvent<HTMLButtonElement>) => {
    onSelect(event.currentTarget.value);
  }, [ onSelect ]);

  return (
    <ScanTableCard
      title={ `${ rooms.length } inference rooms` }
      note="Select a room to inspect its market profile. Peer and settlement totals are unavailable in the room view."
    >
      <TableRoot variant="scan" minW="850px" aria-label="Inference rooms">
        <TableHeader>
          <TableRow>
            <TableColumnHeader>Room / model</TableColumnHeader>
            <TableColumnHeader>Host</TableColumnHeader>
            <TableColumnHeader isNumeric>Peers</TableColumnHeader>
            <TableColumnHeader isNumeric>Open asks</TableColumnHeader>
            <TableColumnHeader isNumeric>Settled tokens</TableColumnHeader>
            <TableColumnHeader>State</TableColumnHeader>
          </TableRow>
        </TableHeader>
        <TableBody>
          { rooms.map((room) => (
            <TableRow key={ room.room_id } data-room={ room.room_id }>
              <TableCell>
                <Button variant="plain" size="sm" value={ room.room_id } onClick={ handleRoom } aria-label={ `Open room ${ room.room_id }` }>
                  <Identifier value={ room.room_id }/>
                </Button>
                <Text color="text.secondary" textStyle="xs">Model <Identifier value={ room.model_digest }/></Text>
              </TableCell>
              <TableCell><Identifier value={ room.host_key }/></TableCell>
              <TableCell isNumeric title="Peer count is not included in the room view">—</TableCell>
              <TableCell isNumeric fontFamily="mono">{ room.open_asks.length }</TableCell>
              <TableCell isNumeric title="Settled tokens are available per deal below">—</TableCell>
              <TableCell>{ room.state === 0 ? 'Open' : 'Closed' }</TableCell>
            </TableRow>
          )) }
        </TableBody>
      </TableRoot>
      { rooms.length === 0 && <Text p={ 4 }>There are no inference rooms.</Text> }
    </ScanTableCard>
  );
}

function RoomDetails({ room }: { room: InferenceRoom }) {
  return (
    <ScanTableCard title="Room profile" note={ <Identifier value={ room.room_id }/> }>
      <Box p={ 4 } textStyle="sm">
        <Text>Model: <Identifier value={ room.model_digest }/></Text>
        <Text>Runtime: <Identifier value={ room.runtime_digest }/></Text>
        <Text>Host: <Identifier value={ room.host_key }/></Text>
        <Text>Open asks: { room.open_asks.length } · Open bids: { room.open_bids.length }</Text>
        <Text>Lap size: { count(room.profile.lap_size_tokens) } tokens · Challenge window: { count(room.profile.challenge_window_blocks) } blocks</Text>
        <Text>Minimum bond: { count(room.profile.minimum_bond) }</Text>
      </Box>
    </ScanTableCard>
  );
}

export function InferenceDealPanel({ deal, room }: { deal: InferenceDeal; room?: InferenceRoom }) {
  const profile = room?.room_id === deal.room_id ? room.profile : undefined;
  const totalLayers = deal.layers[deal.layers.length - 1].end_layer;
  const peerBps = profile ? 10000 - profile.host_share_bps - profile.protocol_share_bps : undefined;

  return (
    <ScanTableCard title="Deal" note={ <Identifier value={ deal.deal_id }/> }>
      <Box p={ 4 } textStyle="sm" data-deal={ deal.deal_id }>
        <Text>Room: <Identifier value={ deal.room_id }/></Text>
        <Text>Ask: <Identifier value={ deal.ask_id }/></Text>
        <Text>Roster: <Identifier value={ deal.roster_digest }/></Text>
        <Flex gap={ 6 } flexWrap="wrap" mt={ 3 } fontFamily="mono">
          <Text>Price / 1,000 tokens: { count(deal.agreed_price_per_thousand_tokens) }</Text>
          <Text>Settled tokens: { count(deal.settled_tokens) }</Text>
          <Text>Amount owed: { count(deal.settled_amount_owed) }</Text>
          <Text>{ deal.state === 0 ? 'Open' : 'Settled' }</Text>
        </Flex>
        <Heading level="3" mt={ 4 } mb={ 2 }>Payout split</Heading>
        { profile && peerBps !== undefined ? (
          <Flex gap={ 6 } data-payout-split>
            <Text>Host { percent(profile.host_share_bps) }</Text>
            <Text>Protocol { percent(profile.protocol_share_bps) }</Text>
            <Text>Peer pool { percent(peerBps) }</Text>
          </Flex>
        ) : <Text>Payout shares are unavailable until the room profile loads.</Text> }
      </Box>
      <TableRoot variant="scan" minW="620px" aria-label="Deal layer split">
        <TableHeader>
          <TableRow>
            <TableColumnHeader>Bid</TableColumnHeader>
            <TableColumnHeader isNumeric>First layer</TableColumnHeader>
            <TableColumnHeader isNumeric>End layer (exclusive)</TableColumnHeader>
            <TableColumnHeader isNumeric>Peer pool weight</TableColumnHeader>
          </TableRow>
        </TableHeader>
        <TableBody>
          { deal.layers.map((layer) => (
            <TableRow key={ layer.bid_id } data-bid={ layer.bid_id }>
              <TableCell><Identifier value={ layer.bid_id }/></TableCell>
              <TableCell isNumeric>{ layer.first_layer }</TableCell>
              <TableCell isNumeric>{ layer.end_layer }</TableCell>
              <TableCell isNumeric fontFamily="mono">{ layer.end_layer - layer.first_layer } / { totalLayers }</TableCell>
            </TableRow>
          )) }
        </TableBody>
      </TableRoot>
    </ScanTableCard>
  );
}

export function InferenceReceiptsTable({ receipts }: { receipts: Array<InferenceReceipt> }) {
  return (
    <ScanTableCard title={ `${ receipts.length } receipt batches` } note="Countersigned lap batches for the selected deal">
      <TableRoot variant="scan" minW="660px" aria-label="Inference receipt batches">
        <TableHeader>
          <TableRow>
            <TableColumnHeader>Batch digest</TableColumnHeader>
            <TableColumnHeader isNumeric>First lap</TableColumnHeader>
            <TableColumnHeader isNumeric>Last lap</TableColumnHeader>
            <TableColumnHeader isNumeric>Cumulative tokens</TableColumnHeader>
            <TableColumnHeader isNumeric>Countersign quorum</TableColumnHeader>
          </TableRow>
        </TableHeader>
        <TableBody>
          { receipts.map((receipt) => (
            <TableRow key={ receipt.batch_digest } data-receipt={ receipt.batch_digest }>
              <TableCell><Identifier value={ receipt.batch_digest }/></TableCell>
              <TableCell isNumeric fontFamily="mono">{ count(receipt.first_lap_index) }</TableCell>
              <TableCell isNumeric fontFamily="mono">{ count(receipt.last_lap_index) }</TableCell>
              <TableCell isNumeric fontFamily="mono">{ count(receipt.cumulative_tokens) }</TableCell>
              <TableCell isNumeric>{ receipt.countersign_quorum }</TableCell>
            </TableRow>
          )) }
        </TableBody>
      </TableRoot>
      { receipts.length === 0 && <Text p={ 4 }>There are no receipt batches for this deal.</Text> }
    </ScanTableCard>
  );
}

export default function PaxeerXInference() {
  const [ selectedRoom, setSelectedRoom ] = React.useState<string>();
  const [ dealInput, setDealInput ] = React.useState('');
  const [ dealId, setDealId ] = React.useState<string>();
  const [ inputError, setInputError ] = React.useState(false);
  const rooms = useQuery({
    queryKey: inferenceQueryKey('px_getInferenceRooms'),
    queryFn: ({ signal }) => paxeerXInference('px_getInferenceRooms', undefined, signal),
    staleTime: 15000,
    retry: false,
  });
  const deal = useQuery({
    queryKey: inferenceQueryKey('px_getInferenceDeal', dealId),
    queryFn: ({ signal }) => paxeerXInference('px_getInferenceDeal', dealId, signal),
    enabled: Boolean(dealId),
    retry: false,
  });
  const roomId = selectedRoom ?? deal.data?.room_id;
  const room = useQuery({
    queryKey: inferenceQueryKey('px_getInferenceRoom', roomId),
    queryFn: ({ signal }) => paxeerXInference('px_getInferenceRoom', roomId, signal),
    enabled: Boolean(roomId),
    retry: false,
  });
  const dealRoomId = deal.data?.room_id;
  const dealRoom = useQuery({
    queryKey: inferenceQueryKey('px_getInferenceRoom', dealRoomId),
    queryFn: ({ signal }) => paxeerXInference('px_getInferenceRoom', dealRoomId, signal),
    enabled: Boolean(dealRoomId),
    retry: false,
  });
  const receipts = useQuery({
    queryKey: inferenceQueryKey('px_getInferenceReceipts', dealId),
    queryFn: ({ signal }) => paxeerXInference('px_getInferenceReceipts', dealId, signal),
    enabled: Boolean(deal.data),
    retry: false,
  });

  const refetchDeal = deal.refetch;
  const handleDeal = React.useCallback((event: React.FormEvent) => {
    event.preventDefault();
    const id = dealInput.trim();
    setInputError(!isInferenceId(id));
    if (isInferenceId(id)) {
      if (id === dealId) void refetchDeal();
      else setDealId(id);
    }
  }, [ dealInput, dealId, refetchDeal ]);
  const handleDealInput = React.useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    setDealInput(event.target.value);
  }, []);

  return (
    <Flex direction="column" gap={ 5 }>
      <Box>
        <Heading level="1">Inference</Heading>
        <Text mt={ 2 } color="text.secondary">Room markets, layer assignments and settled receipt batches.</Text>
      </Box>
      { rooms.isPending && <Text role="status">Loading inference rooms…</Text> }
      { rooms.isError && <Text role="alert">Could not load inference rooms. Please try again.</Text> }
      { rooms.data && <InferenceRoomsTable rooms={ rooms.data } onSelect={ setSelectedRoom }/> }
      { roomId && room.isPending && <Text role="status">Loading room profile…</Text> }
      { roomId && room.isError && <Text role="alert">Could not load the room profile.</Text> }
      { room.data && <RoomDetails room={ room.data }/> }
      <Box as="form" onSubmit={ handleDeal }>
        <chakra.label htmlFor="inference-deal" display="block" mb={ 2 } textStyle="sm">Deal ID</chakra.label>
        <Flex gap={ 3 } flexWrap="wrap">
          <Input id="inference-deal" value={ dealInput } onChange={ handleDealInput }
            placeholder="64 lowercase hexadecimal characters" fontFamily="mono" maxLength={ 64 }
            aria-invalid={ inputError } aria-describedby={ inputError ? 'inference-deal-error' : undefined } flex="1" minW="240px"/>
          <Button type="submit">Look up deal</Button>
        </Flex>
        { inputError && <Text id="inference-deal-error" role="alert">Enter a nonzero deal ID of 64 lowercase hexadecimal characters.</Text> }
      </Box>
      { dealId && deal.isPending && <Text role="status">Loading deal…</Text> }
      { dealId && deal.isError && <Text role="alert">Could not load this deal. Check the ID and try again.</Text> }
      { deal.data && <InferenceDealPanel deal={ deal.data } room={ dealRoom.data }/> }
      { deal.data && dealRoom.isError && <Text role="alert">Could not load the payout profile.</Text> }
      { deal.data && receipts.isPending && <Text role="status">Loading receipt batches…</Text> }
      { deal.data && receipts.isError && <Text role="alert">Could not load receipt batches.</Text> }
      { receipts.data && deal.data && <InferenceReceiptsTable receipts={ receipts.data }/> }
    </Flex>
  );
}
