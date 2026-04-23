// app/lib/features/properties/screens/property_list_screen.dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../models/property.dart';
import '../providers/property_list_provider.dart';
import '../widgets/property_card.dart';

class PropertyListScreen extends ConsumerStatefulWidget {
  const PropertyListScreen({super.key});

  @override
  ConsumerState<PropertyListScreen> createState() => _PropertyListScreenState();
}

class _PropertyListScreenState extends ConsumerState<PropertyListScreen> {
  final _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    // Initial load.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(propertyListProvider.notifier).refresh();
    });
    // Infinite scroll — load more when 80% through.
    _scrollController.addListener(() {
      final pos = _scrollController.position;
      if (pos.pixels >= pos.maxScrollExtent * 0.8) {
        ref.read(propertyListProvider.notifier).fetchNext();
      }
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(propertyListProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Properties'),
        actions: [
          IconButton(
            icon: const Icon(Icons.filter_list),
            onPressed: () => _showFilterSheet(context),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: () => context.push('/properties/add'),
        child: const Icon(Icons.add),
      ),
      body: RefreshIndicator(
        onRefresh: () => ref.read(propertyListProvider.notifier).refresh(),
        child: _buildBody(state),
      ),
    );
  }

  Widget _buildBody(PropertyListState state) {
    if (state.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state.error != null && state.properties.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Text('Error: ${state.error}'),
            const SizedBox(height: 12),
            ElevatedButton(
              onPressed: () => ref.read(propertyListProvider.notifier).refresh(),
              child: const Text('Retry'),
            ),
          ],
        ),
      );
    }
    if (state.properties.isEmpty) {
      return const Center(child: Text('No properties found.'));
    }

    return ListView.builder(
      controller: _scrollController,
      padding: const EdgeInsets.all(12),
      itemCount: state.properties.length + (state.isLoadingMore ? 1 : 0),
      itemBuilder: (context, index) {
        if (index == state.properties.length) {
          return const Padding(
            padding: EdgeInsets.symmetric(vertical: 16),
            child: Center(child: CircularProgressIndicator()),
          );
        }
        final prop = state.properties[index];
        return Padding(
          padding: const EdgeInsets.only(bottom: 10),
          child: PropertyCard(
            property: prop,
            onTap: () => context.push('/properties/${prop.id}'),
          ),
        );
      },
    );
  }

  void _showFilterSheet(BuildContext context) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => const _FilterSheet(),
    );
  }
}

// ── Filter bottom sheet ───────────────────────────────────────────────────────

class _FilterSheet extends ConsumerStatefulWidget {
  const _FilterSheet();

  @override
  ConsumerState<_FilterSheet> createState() => _FilterSheetState();
}

class _FilterSheetState extends ConsumerState<_FilterSheet> {
  late FilterParams _draft;

  @override
  void initState() {
    super.initState();
    _draft = ref.read(propertyListProvider).filters;
  }

  @override
  Widget build(BuildContext context) {
    return DraggableScrollableSheet(
      expand: false,
      initialChildSize: 0.6,
      maxChildSize: 0.9,
      builder: (_, sc) => Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
        child: ListView(
          controller: sc,
          children: [
            Center(
              child: Container(
                width: 40,
                height: 4,
                margin: const EdgeInsets.only(bottom: 16),
                decoration: BoxDecoration(
                  color: Colors.grey[300],
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            Text('Filters', style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: 16),

            // Category toggle
            Text('Category', style: Theme.of(context).textTheme.labelLarge),
            const SizedBox(height: 8),
            ToggleButtons(
              isSelected: [
                _draft.category == 'BUYING',
                _draft.category == 'SELLING',
                _draft.category == null,
              ],
              onPressed: (i) => setState(() {
                _draft = _draft.copyWith(
                    category: i == 0 ? 'BUYING' : i == 1 ? 'SELLING' : null);
              }),
              children: const [Text('BUYING'), Text('SELLING'), Text('All')],
            ),
            const SizedBox(height: 16),

            // Property type chips
            Text('Type', style: Theme.of(context).textTheme.labelLarge),
            const SizedBox(height: 8),
            Wrap(
              spacing: 8,
              children: ['PLOT', 'SHOP', 'FLAT', 'OTHER'].map((t) {
                final selected = _draft.propertyType == t;
                return FilterChip(
                  label: Text(t),
                  selected: selected,
                  onSelected: (_) => setState(() {
                    _draft = _draft.copyWith(
                        propertyType: selected ? null : t);
                  }),
                );
              }).toList(),
            ),
            const SizedBox(height: 16),

            // Direct owner switch
            SwitchListTile(
              title: const Text('Direct Owner Only'),
              value: _draft.isDirectOwner ?? false,
              onChanged: (v) =>
                  setState(() => _draft = _draft.copyWith(isDirectOwner: v)),
            ),
            const SizedBox(height: 24),

            // Apply / Reset
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: () {
                      setState(() => _draft = const FilterParams());
                    },
                    child: const Text('Reset'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: ElevatedButton(
                    onPressed: () {
                      ref.read(propertyListProvider.notifier).applyFilters(_draft);
                      Navigator.pop(context);
                    },
                    child: const Text('Apply'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
