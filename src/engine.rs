//! One request through the engine: prepare, coverage gate, fight.

use std::panic::{catch_unwind, AssertUnwindSafe};

use forever_engine::{
    contracts::prepared_v2::PreparedV2, prepare_json, simulate_prepared_gated, Gated, PrepareError,
};
use serde_json::Value;

/// The `RaidSimResult` of a complete `RaidSimRequest`, or why the engine did not run it.
pub fn simulate(request: &Value) -> Result<Value, String> {
    let bytes = serde_json::to_vec(request).map_err(|e| e.to_string())?;
    // The engine reports a defect in its preparation by panicking.
    catch_unwind(AssertUnwindSafe(|| run(&bytes)))
        .unwrap_or_else(|_| Err("the Forever engine failed on this character".into()))
}

fn run(request: &[u8]) -> Result<Value, String> {
    let prepared = prepare_json(request, "simhammer").map_err(|e| match e {
        PrepareError::Invalid(reason) => format!("request rejected: {reason}"),
        PrepareError::Refused(refusal) => unsupported(&refusal.reason),
        PrepareError::Fault(reason) => {
            format!("the Forever engine failed on this character: {reason}")
        }
    })?;
    let prepared: PreparedV2 = serde_json::from_value(prepared).map_err(|e| e.to_string())?;
    match simulate_prepared_gated(&prepared).map_err(|e| e.to_string())? {
        Gated::Simulated(report) => Ok(report.result),
        Gated::Refused(refusals) => {
            let mut reasons: Vec<&str> = refusals.iter().map(|r| r.reason.as_str()).collect();
            reasons.dedup();
            reasons.truncate(3);
            Err(unsupported(&reasons.join("; ")))
        }
    }
}

fn unsupported(reason: &str) -> String {
    format!("the Forever engine can't sim this yet: {reason}")
}
