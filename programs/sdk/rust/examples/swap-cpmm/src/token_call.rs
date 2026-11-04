use layerx_program_sdk::{
    lxt20::Request,
    payments::{PaymentGrant, PreparedProgramAccount, ProgramPaymentCapabilities},
    AccountId, Amount, AssetId, Bytes, Capability, Field, Principal, ProgramError, ProgramId,
    Reason,
};

/// The settlement subset of the canonical LXT-20 wire contract. The caller
/// identity must come from the runtime's immediate-caller context, never calldata.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct NestedTokenCall {
    pub owner: Principal,
    pub spender: Option<Principal>,
    pub to: AccountId,
    pub amount: Amount,
}

impl NestedTokenCall {
    pub fn decode(input: &[u8], immediate_caller: Option<ProgramId>) -> Result<Self, ProgramError> {
        let caller = immediate_caller
            .ok_or_else(|| ProgramError::value(Field::CallInput, Reason::Malformed))?;
        let caller = Principal::new(caller.bytes())?;
        match Request::decode(input)? {
            Request::Transfer { to, amount } => Ok(Self {
                owner: caller,
                spender: None,
                to,
                amount,
            }),
            Request::TransferFrom { owner, to, amount } => Ok(Self {
                owner,
                spender: Some(caller),
                to,
                amount,
            }),
            _ => Err(ProgramError::value(Field::CallInput, Reason::Malformed)),
        }
    }
}

/// Keeps the request and its exact payment authority together so their owner,
/// recipient and ceiling cannot be supplied independently at the call site.
pub struct TokenCall {
    program: ProgramId,
    asset: AssetId,
    input: Bytes<90>,
    settlement: NestedTokenCall,
    source_seed: [u8; 32],
}

impl TokenCall {
    pub fn pull(
        program: ProgramId,
        asset: AssetId,
        pool: ProgramId,
        owner: Principal,
        amount: Amount,
    ) -> Result<Self, ProgramError> {
        let seed = pool.bytes();
        let to = PreparedProgramAccount::new(program, &seed, asset)?.account();
        Self::new(
            program,
            asset,
            pool,
            Request::TransferFrom { owner, to, amount },
        )
    }

    pub fn push(
        program: ProgramId,
        asset: AssetId,
        pool: ProgramId,
        recipient: Principal,
        amount: Amount,
    ) -> Result<Self, ProgramError> {
        let seed = recipient.bytes();
        let to = PreparedProgramAccount::new(program, &seed, asset)?.account();
        Self::new(program, asset, pool, Request::Transfer { to, amount })
    }

    fn new(
        program: ProgramId,
        asset: AssetId,
        pool: ProgramId,
        request: Request,
    ) -> Result<Self, ProgramError> {
        let input = request.encode()?;
        let settlement = NestedTokenCall::decode(input.as_slice(), Some(pool))?;
        Ok(Self {
            program,
            asset,
            input,
            source_seed: settlement.owner.bytes(),
            settlement,
        })
    }

    pub const fn program(&self) -> ProgramId {
        self.program
    }

    pub fn input(&self) -> &[u8] {
        self.input.as_slice()
    }

    pub fn capabilities(&self) -> Result<ProgramPaymentCapabilities<'_, 3>, ProgramError> {
        let source = PreparedProgramAccount::new(self.program, &self.source_seed, self.asset)?;
        let mut grants = ProgramPaymentCapabilities::empty();
        grants.insert(PaymentGrant::Basic(Capability::SharedStorageRead))?;
        grants.insert(PaymentGrant::Basic(Capability::SharedStorageWrite))?;
        grants.insert(PaymentGrant::ProgramSpend {
            account: source,
            to: self.settlement.to,
            maximum: self.settlement.amount,
        })?;
        Ok(grants)
    }
}
