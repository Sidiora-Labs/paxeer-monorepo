#!/usr/bin/env bash
source docker/localnode/scripts/localnode_common.sh

jq '.validators = []' "$PAX_HOME"/config/genesis.json > "$PAX_HOME"/config/tmp_genesis.json
cd "$SHARED_DIR/gentx"
IDX=0
for FILE in *
do
    jq '.validators['$IDX'] |= .+ {}' "$PAX_HOME"/config/tmp_genesis.json > "$PAX_HOME"/config/tmp_genesis_step_1.json && rm "$PAX_HOME"/config/tmp_genesis.json
    KEY=$(jq '.body.messages[0].pubkey.key' $FILE -c)
    DELEGATION=$(jq -r '.body.messages[0].value.amount' $FILE)
    POWER=$(($DELEGATION / 1000000))
    jq '.validators['$IDX'] += {"power":"'$POWER'"}' "$PAX_HOME"/config/tmp_genesis_step_1.json > "$PAX_HOME"/config/tmp_genesis_step_2.json && rm "$PAX_HOME"/config/tmp_genesis_step_1.json
    jq '.validators['$IDX'] += {"pub_key":{"type":"tendermint/PubKeyEd25519","value":'$KEY'}}' "$PAX_HOME"/config/tmp_genesis_step_2.json > "$PAX_HOME"/config/tmp_genesis_step_3.json && rm "$PAX_HOME"/config/tmp_genesis_step_2.json
    mv "$PAX_HOME"/config/tmp_genesis_step_3.json "$PAX_HOME"/config/tmp_genesis.json
    IDX=$(($IDX+1))
done

mv "$PAX_HOME"/config/tmp_genesis.json "$PAX_HOME"/config/genesis.json
