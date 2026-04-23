// app/lib/features/properties/screens/property_detail_screen.dart
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import 'package:intl/intl.dart';

import '../models/property.dart';
import '../providers/property_detail_provider.dart';
import '../widgets/share_bottom_sheet.dart';

final _inrFormat = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);

class PropertyDetailScreen extends ConsumerWidget {
  const PropertyDetailScreen({super.key, required this.propertyId});

  final String propertyId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(propertyDetailProvider(propertyId));

    return Scaffold(
      body: async.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text('Error: $e')),
        data: (prop) => _PropertyDetail(property: prop),
      ),
    );
  }
}

class _PropertyDetail extends StatefulWidget {
  const _PropertyDetail({required this.property});
  final Property property;

  @override
  State<_PropertyDetail> createState() => _PropertyDetailState();
}

class _PropertyDetailState extends State<_PropertyDetail> {
  int _photoIndex = 0;

  @override
  Widget build(BuildContext context) {
    final prop = widget.property;
    final confirmedPhotos =
        prop.photos.where((p) => p.status == 'CONFIRMED').toList();

    return CustomScrollView(
      slivers: [
        // Photo gallery + app bar
        SliverAppBar(
          expandedHeight: confirmedPhotos.isEmpty ? 80 : 280,
          pinned: true,
          flexibleSpace: FlexibleSpaceBar(
            background: confirmedPhotos.isEmpty
                ? Container(
                    color: Theme.of(context).colorScheme.surfaceContainerHighest,
                    child: const Icon(Icons.home_outlined, size: 80, color: Colors.grey),
                  )
                : _PhotoGallery(
                    photos: confirmedPhotos,
                    currentIndex: _photoIndex,
                    onPageChanged: (i) => setState(() => _photoIndex = i),
                    heroTag: 'property-thumb-${prop.id}',
                  ),
          ),
          actions: [
            IconButton(
              icon: const Icon(Icons.share_outlined),
              tooltip: 'Share',
              onPressed: () => showShareSheet(context, prop.id),
            ),
            IconButton(
              icon: const Icon(Icons.edit_outlined),
              onPressed: () => context.push('/properties/${prop.id}/edit'),
            ),
          ],
        ),

        SliverToBoxAdapter(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Category + type
                Row(
                  children: [
                    _badge(prop.listingCategory,
                        prop.listingCategory == 'SELLING' ? Colors.orange : Colors.green),
                    const SizedBox(width: 8),
                    _badge(prop.propertyType, Colors.blueGrey),
                    if (prop.isDirectOwner) ...[
                      const SizedBox(width: 8),
                      _badge('Direct Owner', Colors.teal),
                    ],
                  ],
                ),
                const SizedBox(height: 16),

                // Price
                Text(
                  _inrFormat.format(prop.price),
                  style: Theme.of(context)
                      .textTheme
                      .headlineSmall
                      ?.copyWith(fontWeight: FontWeight.bold),
                ),
                if (prop.pricePerSqm != null)
                  Text(
                    '${_inrFormat.format(prop.pricePerSqm!)}/sqm',
                    style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                          color: Theme.of(context).colorScheme.primary,
                        ),
                  ),
                const SizedBox(height: 16),

                // Area row
                if (prop.builtUpArea != null || prop.plotArea != null)
                  _detailRow(
                    context,
                    'Area',
                    [
                      if (prop.builtUpArea != null)
                        '${prop.builtUpArea!.toStringAsFixed(0)} sqm built-up',
                      if (prop.plotArea != null)
                        '${prop.plotArea!.toStringAsFixed(0)} sqm plot',
                    ].join('  ·  '),
                  ),

                // Owner info (only shown when backend returns it — SUPER_ADMIN only)
                if (prop.ownerName.isNotEmpty)
                  _detailRow(context, 'Owner', prop.ownerName),
                if (prop.ownerContact.isNotEmpty)
                  _detailRow(context, 'Contact', prop.ownerContact),

                // Description
                if (prop.description != null && prop.description!.isNotEmpty) ...[
                  const SizedBox(height: 12),
                  Text('Description',
                      style: Theme.of(context).textTheme.labelLarge),
                  const SizedBox(height: 4),
                  Text(prop.description!),
                ],

                // Tags
                if (prop.tags.isNotEmpty) ...[
                  const SizedBox(height: 12),
                  Text('Tags', style: Theme.of(context).textTheme.labelLarge),
                  const SizedBox(height: 6),
                  Wrap(
                    spacing: 6,
                    runSpacing: 4,
                    children: prop.tags
                        .map((t) => Chip(label: Text(t), materialTapTargetSize:
                            MaterialTapTargetSize.shrinkWrap))
                        .toList(),
                  ),
                ],

                const SizedBox(height: 24),

                // Embedded map
                Text('Location', style: Theme.of(context).textTheme.labelLarge),
                const SizedBox(height: 8),
                ClipRRect(
                  borderRadius: BorderRadius.circular(12),
                  child: SizedBox(
                    height: 180,
                    child: GoogleMap(
                      initialCameraPosition: CameraPosition(
                        target: LatLng(prop.locationLat, prop.locationLng),
                        zoom: 15,
                      ),
                      markers: {
                        Marker(
                          markerId: MarkerId(prop.id),
                          position: LatLng(prop.locationLat, prop.locationLng),
                        ),
                      },
                      zoomControlsEnabled: false,
                      scrollGesturesEnabled: false,
                      rotateGesturesEnabled: false,
                      tiltGesturesEnabled: false,
                      myLocationButtonEnabled: false,
                    ),
                  ),
                ),
                const SizedBox(height: 32),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _badge(String label, Color color) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(6),
        border: Border.all(color: color.withValues(alpha: 0.4)),
      ),
      child: Text(label,
          style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: color)),
    );
  }

  Widget _detailRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 90,
            child: Text(label,
                style: Theme.of(context)
                    .textTheme
                    .bodySmall
                    ?.copyWith(color: Colors.grey)),
          ),
          Expanded(child: Text(value)),
        ],
      ),
    );
  }
}

class _PhotoGallery extends StatelessWidget {
  const _PhotoGallery({
    required this.photos,
    required this.currentIndex,
    required this.onPageChanged,
    required this.heroTag,
  });

  final List<PropertyPhoto> photos;
  final int currentIndex;
  final ValueChanged<int> onPageChanged;
  final String heroTag;

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        PageView.builder(
          itemCount: photos.length,
          onPageChanged: onPageChanged,
          itemBuilder: (_, i) {
            final widget = CachedNetworkImage(
              imageUrl: photos[i].cdnUrl,
              fit: BoxFit.cover,
              width: double.infinity,
              height: double.infinity,
            );
            return i == 0
                ? Hero(tag: heroTag, child: widget)
                : widget;
          },
        ),
        // Dot indicators
        if (photos.length > 1)
          Positioned(
            bottom: 12,
            left: 0,
            right: 0,
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: List.generate(photos.length, (i) {
                return AnimatedContainer(
                  duration: const Duration(milliseconds: 200),
                  margin: const EdgeInsets.symmetric(horizontal: 3),
                  width: i == currentIndex ? 12 : 6,
                  height: 6,
                  decoration: BoxDecoration(
                    color: i == currentIndex ? Colors.white : Colors.white54,
                    borderRadius: BorderRadius.circular(3),
                  ),
                );
              }),
            ),
          ),
      ],
    );
  }
}
