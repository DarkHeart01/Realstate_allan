// app/lib/features/calculator/screens/calculator_screen.dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/calculator_provider.dart';

class CalculatorScreen extends ConsumerWidget {
  const CalculatorScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(calculatorProvider);
    final notifier = ref.read(calculatorProvider.notifier);

    return Scaffold(
      appBar: AppBar(title: const Text('Brokerage Calculator')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            // ── Mode toggle ────────────────────────────────────────────────
            SegmentedButton<CalcMode>(
              segments: const [
                ButtonSegment(
                  value: CalcMode.sale,
                  label: Text('Sale'),
                  icon: Icon(Icons.sell_outlined),
                ),
                ButtonSegment(
                  value: CalcMode.rental,
                  label: Text('Rental'),
                  icon: Icon(Icons.home_work_outlined),
                ),
              ],
              selected: {state.mode},
              onSelectionChanged: (s) => notifier.setMode(s.first),
            ),
            const SizedBox(height: 24),

            // ── Amount field ───────────────────────────────────────────────
            if (state.mode == CalcMode.sale) ...[
              _AmountField(
                label: 'Property Value (₹)',
                initialValue: state.propertyValue,
                onChanged: notifier.setPropertyValue,
              ),
            ] else ...[
              _AmountField(
                label: 'Monthly Rent (₹)',
                initialValue: state.monthlyRent,
                onChanged: notifier.setMonthlyRent,
              ),
            ],
            const SizedBox(height: 20),

            // ── Commission rate slider ─────────────────────────────────────
            Text(
              'Commission Rate: ${state.commissionRate.toStringAsFixed(1)}%',
              style: Theme.of(context).textTheme.titleSmall,
            ),
            Slider(
              value: state.commissionRate,
              min: 0.5,
              max: 10,
              divisions: 19,
              label: '${state.commissionRate.toStringAsFixed(1)}%',
              onChanged: notifier.setCommissionRate,
            ),
            const SizedBox(height: 20),

            // ── Split ratio ────────────────────────────────────────────────
            Text(
              'Commission Split (optional)',
              style: Theme.of(context).textTheme.titleSmall,
            ),
            const SizedBox(height: 8),
            Wrap(
              spacing: 8,
              children: ['50:50', '60:40', '70:30', '75:25'].map((preset) {
                final active = state.splitRatio == preset;
                return ChoiceChip(
                  label: Text(preset),
                  selected: active,
                  onSelected: (_) {
                    notifier.setSplitRatio(active ? '' : preset);
                  },
                );
              }).toList(),
            ),
            const SizedBox(height: 4),
            TextField(
              decoration: const InputDecoration(
                hintText: 'Custom (e.g. 60:40)',
                isDense: true,
              ),
              onChanged: notifier.setSplitRatio,
              controller: TextEditingController(text: state.splitRatio)
                ..selection = TextSelection.collapsed(
                    offset: state.splitRatio.length),
            ),
            const SizedBox(height: 28),

            // ── Result card ────────────────────────────────────────────────
            if (state.isLoading)
              const Center(child: CircularProgressIndicator())
            else if (state.error != null)
              _ErrorCard(message: state.error!)
            else if (state.hasResult)
              _ResultCard(state: state),
          ],
        ),
      ),
    );
  }
}

// ── Sub-widgets ───────────────────────────────────────────────────────────────

class _AmountField extends StatefulWidget {
  const _AmountField({
    required this.label,
    required this.initialValue,
    required this.onChanged,
  });
  final String label;
  final double initialValue;
  final ValueChanged<double> onChanged;

  @override
  State<_AmountField> createState() => _AmountFieldState();
}

class _AmountFieldState extends State<_AmountField> {
  late final TextEditingController _ctrl;

  @override
  void initState() {
    super.initState();
    final text = widget.initialValue > 0
        ? widget.initialValue.toStringAsFixed(0)
        : '';
    _ctrl = TextEditingController(text: text);
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: _ctrl,
      keyboardType: TextInputType.number,
      decoration: InputDecoration(
        labelText: widget.label,
        border: const OutlineInputBorder(),
        prefixText: '₹ ',
      ),
      onChanged: (v) {
        final parsed = double.tryParse(v.replaceAll(',', ''));
        if (parsed != null) widget.onChanged(parsed);
      },
    );
  }
}

class _ResultCard extends StatelessWidget {
  const _ResultCard({required this.state});
  final CalculatorState state;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Card(
      color: cs.primaryContainer,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Total Brokerage',
              style: Theme.of(context).textTheme.labelLarge,
            ),
            const SizedBox(height: 4),
            Text(
              state.totalCommissionFormatted ?? '',
              style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                    color: cs.onPrimaryContainer,
                    fontWeight: FontWeight.bold,
                  ),
            ),
            if (state.splitA != null && state.splitB != null) ...[
              const Divider(height: 24),
              _SplitRow(label: 'Side A', formatted: state.splitAFormatted ?? ''),
              const SizedBox(height: 4),
              _SplitRow(label: 'Side B', formatted: state.splitBFormatted ?? ''),
            ],
          ],
        ),
      ),
    );
  }
}

class _SplitRow extends StatelessWidget {
  const _SplitRow({required this.label, required this.formatted});
  final String label;
  final String formatted;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label, style: Theme.of(context).textTheme.bodyMedium),
        Text(
          formatted,
          style: Theme.of(context)
              .textTheme
              .bodyMedium
              ?.copyWith(fontWeight: FontWeight.w600),
        ),
      ],
    );
  }
}

class _ErrorCard extends StatelessWidget {
  const _ErrorCard({required this.message});
  final String message;

  @override
  Widget build(BuildContext context) {
    return Card(
      color: Theme.of(context).colorScheme.errorContainer,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Text(
          message,
          style: TextStyle(
            color: Theme.of(context).colorScheme.onErrorContainer,
          ),
        ),
      ),
    );
  }
}
