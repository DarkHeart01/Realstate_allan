// app/lib/features/properties/models/property.dart
import 'package:intl/intl.dart';

final _inrFormat = NumberFormat.currency(
  locale: 'en_IN',
  symbol: '₹',
  decimalDigits: 0,
);

class Property {
  final String id;
  final String listingCategory;
  final String propertyType;
  final String ownerName;
  final String ownerContact;
  final double price;
  final double? plotArea;
  final double? builtUpArea;
  final double locationLat;
  final double locationLng;
  final String? description;
  final List<String> tags;
  final bool isDirectOwner;
  final String? assignedBrokerId;
  final String createdBy;
  final DateTime createdAt;
  final DateTime updatedAt;
  final List<PropertyPhoto> photos;

  const Property({
    required this.id,
    required this.listingCategory,
    required this.propertyType,
    required this.ownerName,
    required this.ownerContact,
    required this.price,
    this.plotArea,
    this.builtUpArea,
    required this.locationLat,
    required this.locationLng,
    this.description,
    required this.tags,
    required this.isDirectOwner,
    this.assignedBrokerId,
    required this.createdBy,
    required this.createdAt,
    required this.updatedAt,
    this.photos = const [],
  });

  factory Property.fromJson(Map<String, dynamic> json) {
    return Property(
      id: json['id'] as String,
      listingCategory: json['listing_category'] as String,
      propertyType: json['property_type'] as String,
      ownerName: (json['owner_name'] as String?) ?? '',
      ownerContact: (json['owner_contact'] as String?) ?? '',
      price: (json['price'] as num).toDouble(),
      plotArea: (json['plot_area'] as num?)?.toDouble(),
      builtUpArea: (json['built_up_area'] as num?)?.toDouble(),
      locationLat: (json['location_lat'] as num).toDouble(),
      locationLng: (json['location_lng'] as num).toDouble(),
      description: json['description'] as String?,
      tags: (json['tags'] as List<dynamic>?)?.cast<String>() ?? [],
      isDirectOwner: (json['is_direct_owner'] as bool?) ?? false,
      assignedBrokerId: json['assigned_broker_id'] as String?,
      createdBy: json['created_by'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
      photos: (json['photos'] as List<dynamic>?)
              ?.map((e) => PropertyPhoto.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }

  /// Indian-formatted price string e.g. "₹50,00,000".
  String get priceFormatted => _inrFormat.format(price);

  /// Price per square metre, using built_up_area first, then plot_area.
  double? get pricePerSqm {
    final area = builtUpArea ?? plotArea;
    if (area == null || area == 0) return null;
    return price / area;
  }

  /// First confirmed photo CDN URL, if any.
  String? get thumbnailUrl {
    for (final p in photos) {
      if (p.status == 'CONFIRMED') return p.cdnUrl;
    }
    return null;
  }
}

class PropertyPhoto {
  final String id;
  final String propertyId;
  final String cdnUrl;
  final String gcsKey;
  final int displayOrder;
  final String status;
  final DateTime createdAt;

  const PropertyPhoto({
    required this.id,
    required this.propertyId,
    required this.cdnUrl,
    required this.gcsKey,
    required this.displayOrder,
    required this.status,
    required this.createdAt,
  });

  factory PropertyPhoto.fromJson(Map<String, dynamic> json) {
    return PropertyPhoto(
      id: json['id'] as String,
      propertyId: json['property_id'] as String,
      cdnUrl: json['cdn_url'] as String,
      gcsKey: json['gcs_key'] as String,
      displayOrder: (json['display_order'] as int?) ?? 0,
      status: json['status'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
