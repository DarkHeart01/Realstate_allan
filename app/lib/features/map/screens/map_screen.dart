// app/lib/features/map/screens/map_screen.dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';

import '../../properties/models/property.dart';
import '../../properties/widgets/property_card.dart';
import '../providers/map_provider.dart';

class MapScreen extends ConsumerStatefulWidget {
  const MapScreen({super.key});

  @override
  ConsumerState<MapScreen> createState() => _MapScreenState();
}

class _MapScreenState extends ConsumerState<MapScreen> {
  GoogleMapController? _mapController;
  double _zoom = 12;
  Property? _selected;

  static const _initialPosition = CameraPosition(
    target: LatLng(19.0760, 72.8777), // Mumbai
    zoom: 12,
  );

  @override
  Widget build(BuildContext context) {
    final mapState = ref.watch(mapProvider);
    final filters = ref.watch(mapFilterProvider);

    final markers = buildMarkers(
      properties: mapState.properties,
      zoom: _zoom,
      onTap: (p) => setState(() => _selected = p),
    );

    return Scaffold(
      body: Stack(
        children: [
          GoogleMap(
            initialCameraPosition: _initialPosition,
            markers: markers,
            myLocationEnabled: true,
            myLocationButtonEnabled: false,
            zoomControlsEnabled: false,
            onMapCreated: (ctrl) => _mapController = ctrl,
            onCameraMove: (pos) => _zoom = pos.zoom,
            onCameraIdle: () async {
              if (_mapController == null) return;
              final bounds = await _mapController!.getVisibleRegion();
              ref.read(mapProvider.notifier).loadForBounds(bounds, filters);
            },
            onTap: (_) => setState(() => _selected = null),
          ),

          // Loading indicator
          if (mapState.isLoading)
            const Positioned(
              top: 60,
              left: 0,
              right: 0,
              child: Center(
                child: _MapChip(label: 'Loading…'),
              ),
            ),

          // Error banner
          if (mapState.error != null)
            Positioned(
              top: 60,
              left: 16,
              right: 16,
              child: _MapChip(
                label: mapState.error!,
                color: Theme.of(context).colorScheme.errorContainer,
              ),
            ),

          // Property count chip
          if (!mapState.isLoading && mapState.properties.isNotEmpty)
            Positioned(
              top: 60,
              left: 0,
              right: 0,
              child: Center(
                child: _MapChip(
                  label: '${mapState.properties.length} properties',
                ),
              ),
            ),

          // My location FAB
          Positioned(
            bottom: _selected != null ? 220 : 24,
            right: 16,
            child: FloatingActionButton.small(
              heroTag: 'my_location',
              onPressed: () {
                _mapController?.animateCamera(
                  CameraUpdate.newLatLngZoom(
                    _initialPosition.target,
                    _initialPosition.zoom,
                  ),
                );
              },
              child: const Icon(Icons.my_location),
            ),
          ),

          // Selected property bottom sheet
          if (_selected != null)
            Positioned(
              left: 0,
              right: 0,
              bottom: 0,
              child: _PropertyBottomCard(
                property: _selected!,
                onClose: () => setState(() => _selected = null),
                onTap: () => context.push('/properties/${_selected!.id}'),
              ),
            ),
        ],
      ),
    );
  }
}

// ── Sub-widgets ───────────────────────────────────────────────────────────────

class _MapChip extends StatelessWidget {
  const _MapChip({required this.label, this.color});
  final String label;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    return Material(
      elevation: 2,
      borderRadius: BorderRadius.circular(20),
      color: color ?? Theme.of(context).colorScheme.surface,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
        child: Text(label, style: Theme.of(context).textTheme.labelMedium),
      ),
    );
  }
}

class _PropertyBottomCard extends StatelessWidget {
  const _PropertyBottomCard({
    required this.property,
    required this.onClose,
    required this.onTap,
  });

  final Property property;
  final VoidCallback onClose;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(16)),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withOpacity(0.12),
            blurRadius: 10,
            offset: const Offset(0, -2),
          ),
        ],
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const SizedBox(height: 8),
          Container(
            width: 40,
            height: 4,
            decoration: BoxDecoration(
              color: Colors.grey[300],
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          const SizedBox(height: 4),
          Align(
            alignment: Alignment.topRight,
            child: IconButton(
              icon: const Icon(Icons.close),
              onPressed: onClose,
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
            child: PropertyCard(property: property, onTap: onTap),
          ),
        ],
      ),
    );
  }
}
