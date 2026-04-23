// app/lib/features/properties/widgets/property_card.dart
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import '../models/property.dart';

/// Price formatter for Indian locale: ₹50,00,000
final _inrFormat = NumberFormat.currency(
  locale: 'en_IN',
  symbol: '₹',
  decimalDigits: 0,
);

class PropertyCard extends StatelessWidget {
  const PropertyCard({
    super.key,
    required this.property,
    required this.onTap,
  });

  final Property property;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final thumb = property.thumbnailUrl;

    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Thumbnail
            Hero(
              tag: 'property-thumb-${property.id}',
              child: SizedBox(
                width: 110,
                height: 110,
                child: thumb != null
                    ? CachedNetworkImage(
                        imageUrl: thumb,
                        fit: BoxFit.cover,
                        placeholder: (_, __) => const _ThumbPlaceholder(),
                        errorWidget: (_, __, ___) => const _ThumbPlaceholder(),
                      )
                    : const _ThumbPlaceholder(),
              ),
            ),
            // Details
            Expanded(
              child: Padding(
                padding: const EdgeInsets.all(10),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Category + type badges
                    Row(
                      children: [
                        _Badge(
                          label: property.listingCategory,
                          color: property.listingCategory == 'SELLING'
                              ? Colors.orange
                              : Colors.green,
                        ),
                        const SizedBox(width: 6),
                        _Badge(label: property.propertyType, color: Colors.blueGrey),
                      ],
                    ),
                    const SizedBox(height: 6),
                    // Price
                    Text(
                      _inrFormat.format(property.price),
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    // Area
                    if (property.builtUpArea != null || property.plotArea != null)
                      Text(
                        _areaLabel(property),
                        style: theme.textTheme.bodySmall,
                      ),
                    // Price per sqm
                    if (property.pricePerSqm != null)
                      Text(
                        '${_inrFormat.format(property.pricePerSqm!)}/sqm',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.primary,
                        ),
                      ),
                    // Description snippet
                    if (property.description != null &&
                        property.description!.isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.only(top: 4),
                        child: Text(
                          property.description!,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: theme.textTheme.bodySmall,
                        ),
                      ),
                    // Tags
                    if (property.tags.isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.only(top: 6),
                        child: Wrap(
                          spacing: 4,
                          runSpacing: 2,
                          children: property.tags
                              .take(3)
                              .map((t) => _TagChip(label: t))
                              .toList(),
                        ),
                      ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _areaLabel(Property p) {
    if (p.builtUpArea != null) {
      return '${p.builtUpArea!.toStringAsFixed(0)} sqm built-up';
    }
    return '${p.plotArea!.toStringAsFixed(0)} sqm plot';
  }
}

class _Badge extends StatelessWidget {
  const _Badge({required this.label, required this.color});
  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: color.withValues(alpha: 0.4)),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 10,
          fontWeight: FontWeight.w600,
          color: color,
        ),
      ),
    );
  }
}

class _TagChip extends StatelessWidget {
  const _TagChip({required this.label});
  final String label;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(label, style: const TextStyle(fontSize: 10)),
    );
  }
}

class _ThumbPlaceholder extends StatelessWidget {
  const _ThumbPlaceholder();

  @override
  Widget build(BuildContext context) {
    return Container(
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      child: const Icon(Icons.home_outlined, size: 40, color: Colors.grey),
    );
  }
}
