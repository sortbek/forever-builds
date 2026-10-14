// Reads each spec's `defaults` block from the engine's spec.ts/presets.ts without running them and
// writes the parts a headless sim needs as protojson, plus a hash of each spec's rotation choice.
// Usage: node scripts/extract-defaults.mjs <engine dir> <out dir> <rotation lock file>
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import ts from 'typescript';

const [engineDir, outDir, lockFile] = process.argv.slice(2);

const SPECS = {
	balanceDruid: 'druid/balance',
	feralCatDruid: 'druid/feralcat',
	feralBearDruid: 'druid/feralbear',
	hunter: 'hunter/dps',
	mage: 'mage/dps',
	protectionPaladin: 'paladin/protection',
	retributionPaladin: 'paladin/retribution',
	dpsPriest: 'priest/dps',
	rogue: 'rogue/dps',
	elementalShaman: 'shaman/elemental',
	enhancementShaman: 'shaman/enhancement',
	warlock: 'warlock/dps',
	dpsWarrior: 'warrior/dps',
	protectionWarrior: 'warrior/protection',
};
const KEYS = ['specOptions', 'consumables', 'raidBuffs', 'partyBuffs', 'individualBuffs', 'debuffs', 'other', 'talents'];
const ROTATION_KEYS = ['autoRotation', 'rotationType', 'aplRotation'];

const files = new Map();
function source(file) {
	if (!files.has(file)) {
		files.set(file, ts.createSourceFile(file, fs.readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true));
	}
	return files.get(file);
}

function fail(sf, node, msg) {
	const { line } = sf.getLineAndCharacterOfPosition(node.getStart(sf));
	throw new Error(`${path.relative(engineDir, sf.fileName)}:${line + 1}: ${msg}: ${node.getText(sf).slice(0, 80)}`);
}

// Relative import bindings: local name -> { file, exported name | '*' }.
function imports(sf) {
	const out = new Map();
	for (const st of sf.statements) {
		if (!ts.isImportDeclaration(st) || !st.importClause) continue;
		const spec = st.moduleSpecifier.text;
		if (!spec.startsWith('.')) continue;
		const file = path.resolve(path.dirname(sf.fileName), spec.endsWith('.ts') ? spec : `${spec}.ts`);
		const nb = st.importClause.namedBindings;
		if (nb && ts.isNamespaceImport(nb)) out.set(nb.name.text, { file, name: '*' });
		if (nb && ts.isNamedImports(nb)) {
			for (const el of nb.elements) out.set(el.name.text, { file, name: (el.propertyName ?? el.name).text });
		}
	}
	return out;
}

function topLevelConst(sf, name) {
	for (const st of sf.statements) {
		if (!ts.isVariableStatement(st)) continue;
		for (const d of st.declarationList.declarations) {
			if (ts.isIdentifier(d.name) && d.name.text === name) return d.initializer;
		}
	}
	return null;
}

function resolveName(sf, name) {
	const init = topLevelConst(sf, name);
	if (init) return [sf, init];
	const imp = imports(sf).get(name);
	if (!imp || imp.name === '*') return null;
	return resolveName(source(imp.file), imp.name);
}

function evaluate(sf, node) {
	if (ts.isParenthesizedExpression(node) || ts.isAsExpression(node) || ts.isSatisfiesExpression(node)) return evaluate(sf, node.expression);
	if (ts.isNumericLiteral(node)) return Number(node.text);
	if (ts.isPrefixUnaryExpression(node) && node.operator === ts.SyntaxKind.MinusToken) return -evaluate(sf, node.operand);
	if (ts.isStringLiteral(node)) return node.text;
	if (node.kind === ts.SyntaxKind.TrueKeyword) return true;
	if (node.kind === ts.SyntaxKind.FalseKeyword) return false;
	if (ts.isArrayLiteralExpression(node)) return node.elements.map(e => evaluate(sf, e));
	if (ts.isObjectLiteralExpression(node)) {
		const out = {};
		for (const p of node.properties) {
			if (ts.isPropertyAssignment(p)) out[p.name.getText(sf)] = evaluate(sf, p.initializer);
			else if (ts.isShorthandPropertyAssignment(p)) out[p.name.text] = evaluate(sf, p.name);
			else if (ts.isSpreadAssignment(p)) Object.assign(out, evaluate(sf, p.expression));
			else fail(sf, p, 'unsupported property');
		}
		return out;
	}
	if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression)) {
		const fn = node.expression.name.text;
		// PresetUtils.makePresetTalents(name, SavedTalents.create({...})) -> { data }
		if (fn === 'makePresetTalents') return { data: evaluate(sf, node.arguments[1]) };
		// Message.create({...}) / Message.create()
		if (fn === 'create') return node.arguments.length ? evaluate(sf, node.arguments[0]) : {};
	}
	if (ts.isIdentifier(node)) {
		const hit = resolveName(sf, node.text);
		if (!hit) fail(sf, node, 'unresolved identifier');
		return evaluate(hit[0], hit[1]);
	}
	if (ts.isPropertyAccessExpression(node) && ts.isIdentifier(node.expression)) {
		const imp = imports(sf).get(node.expression.text);
		if (imp && imp.name === '*') {
			const hit = resolveName(source(imp.file), node.name.text);
			if (!hit) fail(sf, node, 'unresolved export');
			return evaluate(hit[0], hit[1]);
		}
		// Enum.Member: protojson takes the member name.
		return node.name.text;
	}
	if (ts.isPropertyAccessExpression(node)) {
		const base = evaluate(sf, node.expression);
		if (base === null || typeof base !== 'object' || !(node.name.text in base)) fail(sf, node, 'unresolved property');
		return base[node.name.text];
	}
	fail(sf, node, 'unsupported expression');
}

function specConfigProperties(sf) {
	let defaults = null;
	const rotation = [];
	const visit = n => {
		if (ts.isPropertyAssignment(n)) {
			const name = n.name.getText(sf);
			if (name === 'defaults' && ts.isObjectLiteralExpression(n.initializer) && !defaults) defaults = n.initializer;
			if (name === 'autoRotation') rotation.push(n.getText(sf));
		}
		ts.forEachChild(n, visit);
	};
	visit(sf);
	if (!defaults) throw new Error(`${sf.fileName}: no defaults block`);
	return { defaults, rotation };
}

fs.mkdirSync(outDir, { recursive: true });
const lock = {};
for (const [key, dir] of Object.entries(SPECS)) {
	const sf = source(path.join(engineDir, 'ui/specs', dir, 'spec.ts'));
	const { defaults, rotation } = specConfigProperties(sf);
	const out = {};
	for (const p of defaults.properties) {
		const name = p.name?.getText(sf);
		if (ROTATION_KEYS.includes(name)) rotation.push(p.getText(sf));
		if (KEYS.includes(name)) out[name] = evaluate(sf, p.initializer);
	}
	for (const k of KEYS) if (!(k in out)) out[k] = {};
	out.talents = out.talents.talentsString ?? '';
	const other = out.other;
	out.other = {
		reactionTimeMs: other.reactionTime || 100,
		distanceFromTarget: other.distanceFromTarget || 0,
		channelClipDelayMs: other.channelClipDelay || 0,
	};
	fs.writeFileSync(path.join(outDir, `${key}.json`), JSON.stringify(out, null, '\t') + '\n');
	lock[key] = crypto.createHash('sha256').update(rotation.join('\n')).digest('hex');
}
fs.writeFileSync(lockFile, JSON.stringify(lock, null, '\t') + '\n');
console.log(`defaults for ${Object.keys(SPECS).length} specs written to ${outDir}`);
