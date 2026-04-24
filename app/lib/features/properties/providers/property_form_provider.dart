// app/lib/features/properties/providers/property_form_provider.dart
import 'dart:io';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/dio_client.dart';

// ── Form state ────────────────────────────────────────────────────────────────

class PropertyFormState {
  // Step 1
  final String listingCategory;
  final String propertyType;
  final bool isDirectOwner;
  final String? assignedBrokerId;

  // Step 2
  final String locationLat;
  final String locationLng;
  final String description;

  // Step 3
  final String ownerName;
  final String ownerContact;
  final String price;
  final String plotArea;
  final String builtUpArea;

  // Step 4 — photos tracked separately via PhotoUploadState
  final List<PhotoUploadState> photos;

  // Step 5
  final List<String> tags;

  // Meta
  final bool isSubmitting;
  final String? submitError;
  final String? createdPropertyId;

  const PropertyFormState({
    this.listingCategory = 'SELLING',
    this.propertyType = 'FLAT',
    this.isDirectOwner = false,
    this.assignedBrokerId,
    this.locationLat = '',
    this.locationLng = '',
    this.description = '',
    this.ownerName = '',
    this.ownerContact = '',
    this.price = '',
    this.plotArea = '',
    this.builtUpArea = '',
    this.photos = const [],
    this.tags = const [],
    this.isSubmitting = false,
    this.submitError,
    this.createdPropertyId,
  });

  PropertyFormState copyWith({
    String? listingCategory,
    String? propertyType,
    bool? isDirectOwner,
    String? assignedBrokerId,
    String? locationLat,
    String? locationLng,
    String? description,
    String? ownerName,
    String? ownerContact,
    String? price,
    String? plotArea,
    String? builtUpArea,
    List<PhotoUploadState>? photos,
    List<String>? tags,
    bool? isSubmitting,
    String? submitError,
    String? createdPropertyId,
  }) {
    return PropertyFormState(
      listingCategory: listingCategory ?? this.listingCategory,
      propertyType: propertyType ?? this.propertyType,
      isDirectOwner: isDirectOwner ?? this.isDirectOwner,
      assignedBrokerId: assignedBrokerId ?? this.assignedBrokerId,
      locationLat: locationLat ?? this.locationLat,
      locationLng: locationLng ?? this.locationLng,
      description: description ?? this.description,
      ownerName: ownerName ?? this.ownerName,
      ownerContact: ownerContact ?? this.ownerContact,
      price: price ?? this.price,
      plotArea: plotArea ?? this.plotArea,
      builtUpArea: builtUpArea ?? this.builtUpArea,
      photos: photos ?? this.photos,
      tags: tags ?? this.tags,
      isSubmitting: isSubmitting ?? this.isSubmitting,
      submitError: submitError,
      createdPropertyId: createdPropertyId ?? this.createdPropertyId,
    );
  }
}

enum PhotoUploadStatus { pending, uploading, confirmed, failed }

class PhotoUploadState {
  final File file;
  final String? photoId;
  final String? cdnUrl;
  final PhotoUploadStatus status;
  final double progress;

  const PhotoUploadState({
    required this.file,
    this.photoId,
    this.cdnUrl,
    this.status = PhotoUploadStatus.pending,
    this.progress = 0,
  });

  PhotoUploadState copyWith({
    String? photoId,
    String? cdnUrl,
    PhotoUploadStatus? status,
    double? progress,
  }) {
    return PhotoUploadState(
      file: file,
      photoId: photoId ?? this.photoId,
      cdnUrl: cdnUrl ?? this.cdnUrl,
      status: status ?? this.status,
      progress: progress ?? this.progress,
    );
  }
}

// ── Notifier ──────────────────────────────────────────────────────────────────

class PropertyFormNotifier extends StateNotifier<PropertyFormState> {
  PropertyFormNotifier() : super(const PropertyFormState());

  final _dio = createDioClient();

  void update(PropertyFormState Function(PropertyFormState) updater) {
    state = updater(state);
  }

  void addTag(String tag) {
    if (!state.tags.contains(tag)) {
      state = state.copyWith(tags: [...state.tags, tag]);
    }
  }

  void removeTag(String tag) {
    state = state.copyWith(tags: state.tags.where((t) => t != tag).toList());
  }

  void addPhoto(File file) {
    state = state.copyWith(photos: [...state.photos, PhotoUploadState(file: file)]);
  }

  void removePhoto(int index) {
    final updated = [...state.photos];
    updated.removeAt(index);
    state = state.copyWith(photos: updated);
  }

  /// Upload a photo at [index] to GCS via the presign flow.
  /// IMPORTANT: the PUT to GCS uses a plain Dio instance (no Authorization header)
  /// because GCS V4 signed URLs are self-authenticating.
  Future<void> uploadPhoto(String propertyId, int index) async {
    final photo = state.photos[index];
    _updatePhoto(index, photo.copyWith(status: PhotoUploadStatus.uploading, progress: 0));

    try {
      // 1. Get signed URL from API.
      final filename = photo.file.path.split('/').last;
      final ext = filename.split('.').last.toLowerCase();
      final contentType = ext == 'png'
          ? 'image/png'
          : ext == 'webp'
              ? 'image/webp'
              : 'image/jpeg';

      final presignRes = await _dio.post(
        '/api/properties/$propertyId/photos/presign',
        data: {'filename': filename, 'content_type': contentType},
      );
      final presignData = presignRes.data['data'] as Map<String, dynamic>;
      final photoId = presignData['photo_id'] as String;
      final uploadUrl = presignData['upload_url'] as String;
      final cdnUrl = presignData['cdn_url'] as String;

      _updatePhoto(index, photo.copyWith(
        photoId: photoId,
        cdnUrl: cdnUrl,
        status: PhotoUploadStatus.uploading,
        progress: 0.1,
      ));

      // 2. PUT directly to GCS — NO Authorization header (signed URL is self-auth).
      final gcsDio = Dio();
      await gcsDio.put(
        uploadUrl,
        data: photo.file.openRead(),
        options: Options(
          headers: {
            'Content-Type': contentType,
            'Content-Length': await photo.file.length(),
          },
        ),
        onSendProgress: (sent, total) {
          if (total > 0) {
            _updatePhoto(index, state.photos[index].copyWith(
              progress: 0.1 + (sent / total) * 0.8,
            ));
          }
        },
      );

      // 3. Confirm upload with API.
      await _dio.post('/api/properties/$propertyId/photos/$photoId/confirm');

      _updatePhoto(index, state.photos[index].copyWith(
        status: PhotoUploadStatus.confirmed,
        progress: 1.0,
      ));
    } catch (e) {
      _updatePhoto(index, state.photos[index].copyWith(
        status: PhotoUploadStatus.failed,
      ));
    }
  }

  void _updatePhoto(int index, PhotoUploadState updated) {
    final photos = [...state.photos];
    photos[index] = updated;
    state = state.copyWith(photos: photos);
  }

  /// Submit the form. Creates the property, then uploads pending photos.
  Future<void> submit() async {
    // Client-side validation before hitting the network.
    if (state.ownerName.trim().isEmpty) {
      state = state.copyWith(submitError: 'Owner name is required (fill in Step 3)');
      return;
    }
    if (state.ownerContact.trim().isEmpty) {
      state = state.copyWith(submitError: 'Owner contact is required (fill in Step 3)');
      return;
    }
    final parsedPrice = double.tryParse(state.price);
    if (parsedPrice == null || parsedPrice <= 0) {
      state = state.copyWith(submitError: 'Price must be greater than 0 (fill in Step 3)');
      return;
    }

    state = state.copyWith(isSubmitting: true, submitError: null);

    try {
      final body = <String, dynamic>{
        'listing_category': state.listingCategory,
        'property_type': state.propertyType,
        'owner_name': state.ownerName,
        'owner_contact': state.ownerContact,
        'price': double.tryParse(state.price) ?? 0,
        'location_lat': double.tryParse(state.locationLat) ?? 0,
        'location_lng': double.tryParse(state.locationLng) ?? 0,
        'is_direct_owner': state.isDirectOwner,
        'tags': state.tags,
        if (state.description.isNotEmpty) 'description': state.description,
        if (state.plotArea.isNotEmpty) 'plot_area': double.tryParse(state.plotArea),
        if (state.builtUpArea.isNotEmpty) 'built_up_area': double.tryParse(state.builtUpArea),
        if (state.assignedBrokerId != null) 'assigned_broker_id': state.assignedBrokerId,
      };

      final res = await _dio.post('/api/properties', data: body);
      final propertyId = res.data['data']['id'] as String;

      state = state.copyWith(createdPropertyId: propertyId, isSubmitting: false);

      // Upload all selected photos.
      for (var i = 0; i < state.photos.length; i++) {
        await uploadPhoto(propertyId, i);
      }
    } catch (e) {
      String msg = e.toString();
      if (e is DioException) {
        msg = e.response?.data?['message'] as String? ?? 'Submit failed';
      }
      state = state.copyWith(isSubmitting: false, submitError: msg);
    }
  }

  void reset() => state = const PropertyFormState();
}

// ── Provider ──────────────────────────────────────────────────────────────────

final propertyFormProvider =
    StateNotifierProvider<PropertyFormNotifier, PropertyFormState>(
  (ref) => PropertyFormNotifier(),
);
