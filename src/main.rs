//! `simhammer-forever`: the runner SimHammer uses to sim WoW Forever characters. It fills a
//! request with the build's defaults and runs it in the MythicSim Forever engine.

mod bulk;
mod defaults;
mod engine;
#[cfg(test)]
mod tests;

use std::{env, fs, process};

use serde_json::{json, Value};

const USAGE: &str =
    "usage: simhammer-forever sim|bulk|options --in REQUEST.json --out RESULT.json [--max-combinations N] | version";

fn read(path: &str) -> Result<Value, String> {
    let text = fs::read_to_string(path).map_err(|e| format!("read request: {e}"))?;
    serde_json::from_str(&text).map_err(|e| format!("read request: {e}"))
}

fn write(path: &str, value: &Value) -> Result<(), String> {
    fs::write(path, value.to_string()).map_err(|e| format!("write result: {e}"))
}

fn run(args: &[String]) -> Result<(), String> {
    let command = args.first().map(String::as_str).unwrap_or("");
    if command == "version" {
        println!(
            "{}",
            json!({
                "version": env!("CLI_VERSION"),
                "engine_commit": env!("ENGINE_COMMIT"),
                "client_build": env!("CLIENT_BUILD"),
            })
        );
        return Ok(());
    }
    let (mut input, mut output, mut max_combinations) = (None, None, 0usize);
    let mut rest = args.iter().skip(1);
    while let Some(flag) = rest.next() {
        let value = rest.next().ok_or_else(|| format!("{flag} needs a value"))?;
        match flag.as_str() {
            "--in" => input = Some(value.clone()),
            "--out" => output = Some(value.clone()),
            "--max-combinations" if command == "bulk" => {
                max_combinations = value
                    .parse()
                    .map_err(|_| "--max-combinations takes a number")?
            }
            _ => return Err(format!("unknown flag {flag}")),
        }
    }
    let (input, output) = (input.ok_or(USAGE)?, output.ok_or(USAGE)?);
    let mut request = read(&input)?;
    let result = match command {
        "sim" => {
            defaults::apply(&mut request)?;
            engine::simulate(&request)?
        }
        "bulk" => bulk::run(request, max_combinations)?,
        "options" => defaults::options(&request),
        _ => return Err(USAGE.into()),
    };
    write(&output, &result)
}

fn main() {
    let args: Vec<String> = env::args().skip(1).collect();
    if let Err(message) = run(&args) {
        eprintln!("error: {message}");
        process::exit(1);
    }
}
