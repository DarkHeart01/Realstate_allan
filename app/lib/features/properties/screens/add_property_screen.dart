// app/lib/features/properties/screens/add_property_screen.dart
import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import 'package:image_picker/image_picker.dart';
import 'package:intl/intl.dart';

import '../../../core/network/dio_client.dart';
import '../providers/property_form_provider.dart';

final _inrFormat =
    NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);

class AddPropertyScreen extends ConsumerStatefulWidget {
  const AddPropertyScreen({super.key, this.editPropertyId});

  /// When non-null, the form pre-populates for editing.
  final String? editPropertyId;

  @override
  ConsumerState<AddPropertyScreen> createState() => _AddPropertyScreenState();
}

class _AddPropertyScreenState extends ConsumerState<AddPropertyScreen> {
  final _pageController = PageController();
  int _currentStep = 0;
  static const _totalSteps = 6;

  @override
  void initState() {
    super.initState();
    if (widget.editPropertyId != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        ref.read(propertyFormProvider.notifier).loadForEdit(widget.editPropertyId!);
      });
    }
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  void _next() {
    if (_currentStep < _totalSteps - 1) {
      setState(() => _currentStep++);
      _pageController.nextPage(
          duration: const Duration(milliseconds: 300), curve: Curves.easeInOut);
    }
  }

  void _back() {
    if (_currentStep > 0) {
      setState(() => _currentStep--);
      _pageController.previousPage(
          duration: const Duration(milliseconds: 300), curve: Curves.easeInOut);
    } else {
      context.pop();
    }
  }

  @override
  Widget build(BuildContext context) {
    final stepTitles = [
      'Category & Type',
      'Location',
      'Details & Pricing',
      'Photos',
      'Tags',
      'Review & Submit',
    ];

    return Scaffold(
      appBar: AppBar(
        leading: BackButton(onPressed: _back),
        title: Text(stepTitles[_currentStep]),
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(4),
          child: LinearProgressIndicator(
            value: (_currentStep + 1) / _totalSteps,
          ),
        ),
      ),
      body: PageView(
        controller: _pageController,
        physics: const NeverScrollableScrollPhysics(),
        children: [
          _Step1(onNext: _next),
          _Step2(onNext: _next),
          _Step3(onNext: _next),
          _Step4(onNext: _next, editPropertyId: widget.editPropertyId),
          _Step5(onNext: _next),
          _Step6(onSuccess: () {
            context.pop();
            ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(content: Text('Property created successfully')),
            );
            ref.read(propertyFormProvider.notifier).reset();
          }),
        ],
      ),
    );
  }
}

// ── Step 1: Category & Type ───────────────────────────────────────────────────

class _Step1 extends ConsumerWidget {
  const _Step1({required this.onNext});
  final VoidCallback onNext;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final form = ref.watch(propertyFormProvider);
    final notifier = ref.read(propertyFormProvider.notifier);

    return _StepWrapper(
      onNext: onNext,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _label('Listing Category'),
          Row(
            children: ['BUYING', 'SELLING'].map((cat) {
              final sel = form.listingCategory == cat;
              return Expanded(
                child: Padding(
                  padding: const EdgeInsets.only(right: 8),
                  child: OutlinedButton(
                    style: OutlinedButton.styleFrom(
                      backgroundColor: sel
                          ? Theme.of(context).colorScheme.primaryContainer
                          : null,
                    ),
                    onPressed: () => notifier
                        .update((s) => s.copyWith(listingCategory: cat)),
                    child: Text(cat),
                  ),
                ),
              );
            }).toList(),
          ),
          const SizedBox(height: 24),
          _label('Property Type'),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: ['PLOT', 'SHOP', 'FLAT', 'OTHER'].map((t) {
              final sel = form.propertyType == t;
              return ChoiceChip(
                label: Text(t),
                selected: sel,
                showCheckmark: false,
                onSelected: (_) =>
                    notifier.update((s) => s.copyWith(propertyType: t)),
              );
            }).toList(),
          ),
          const SizedBox(height: 24),
          SwitchListTile(
            title: const Text('Direct Owner'),
            subtitle: const Text('Listing is from the property owner directly'),
            value: form.isDirectOwner,
            onChanged: (v) =>
                notifier.update((s) => s.copyWith(isDirectOwner: v)),
          ),
        ],
      ),
    );
  }
}

// ── Step 2: Location ──────────────────────────────────────────────────────────

class _Step2 extends ConsumerStatefulWidget {
  const _Step2({required this.onNext});
  final VoidCallback onNext;

  @override
  ConsumerState<_Step2> createState() => _Step2State();
}

class _Step2State extends ConsumerState<_Step2> {
  // Default to Mumbai CBD if no previous value.
  static const _defaultLat = 19.0760;
  static const _defaultLng = 72.8777;

  late TextEditingController _lat, _lng, _desc;
  GoogleMapController? _mapController;
  Timer? _reverseGeocodeDebounce;
  String _locationLabel = '';

  @override
  void initState() {
    super.initState();
    final s = ref.read(propertyFormProvider);
    _lat = TextEditingController(text: s.locationLat);
    _lng = TextEditingController(text: s.locationLng);
    _desc = TextEditingController(text: s.description);
  }

  @override
  void dispose() {
    _lat.dispose();
    _lng.dispose();
    _desc.dispose();
    _reverseGeocodeDebounce?.cancel();
    super.dispose();
  }

  double get _currentLat =>
      double.tryParse(_lat.text) ?? _defaultLat;
  double get _currentLng =>
      double.tryParse(_lng.text) ?? _defaultLng;

  void _onCameraIdle(CameraPosition pos) {
    // Sync text fields from the map.
    final lat = pos.target.latitude.toStringAsFixed(6);
    final lng = pos.target.longitude.toStringAsFixed(6);
    setState(() {
      _lat.text = lat;
      _lng.text = lng;
      _locationLabel = '$lat, $lng';
    });

    // Debounced reverse-geocode label update (400 ms) — just updates the
    // display label; actual coords are already captured above.
    _reverseGeocodeDebounce?.cancel();
    _reverseGeocodeDebounce = Timer(const Duration(milliseconds: 400), () {
      setState(() => _locationLabel = '$lat, $lng');
    });
  }

  void _onLatTextChanged(String v) {
    final lat = double.tryParse(v);
    final lng = double.tryParse(_lng.text);
    if (lat != null && lng != null && _mapController != null) {
      _mapController!.animateCamera(
        CameraUpdate.newLatLng(LatLng(lat, lng)),
      );
    }
  }

  void _onLngTextChanged(String v) {
    final lat = double.tryParse(_lat.text);
    final lng = double.tryParse(v);
    if (lat != null && lng != null && _mapController != null) {
      _mapController!.animateCamera(
        CameraUpdate.newLatLng(LatLng(lat, lng)),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final notifier = ref.read(propertyFormProvider.notifier);

    return _StepWrapper(
      onNext: () {
        notifier.update((s) => s.copyWith(
              locationLat: _lat.text,
              locationLng: _lng.text,
              description: _desc.text,
            ));
        widget.onNext();
      },
      child: Column(
        children: [
          // ── Map picker ─────────────────────────────────────────────────
          if (!kIsWeb)
            Stack(
              alignment: Alignment.center,
              children: [
                SizedBox(
                  height: 240,
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(12),
                    child: GoogleMap(
                      initialCameraPosition: CameraPosition(
                        target: LatLng(_currentLat, _currentLng),
                        zoom: 15,
                      ),
                      onMapCreated: (ctrl) => _mapController = ctrl,
                      onCameraIdle: () {},
                      onCameraMove: _onCameraIdle,
                      zoomControlsEnabled: true,
                      myLocationButtonEnabled: true,
                      myLocationEnabled: true,
                    ),
                  ),
                ),
                // Centre pin
                const IgnorePointer(
                  child: Icon(Icons.location_pin, size: 40, color: Colors.red),
                ),
              ],
            )
          else
            Container(
              height: 60,
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.surfaceContainerHighest,
                borderRadius: BorderRadius.circular(12),
              ),
              alignment: Alignment.center,
              child: const Text('Enter coordinates manually below'),
            ),
          if (_locationLabel.isNotEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 6),
              child: Text(
                _locationLabel,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: Theme.of(context).colorScheme.primary,
                    ),
              ),
            ),
          const SizedBox(height: 8),

          // ── Fallback text fields (bidirectionally synced) ───────────────
          TextField(
            controller: _lat,
            decoration: const InputDecoration(
              labelText: 'Latitude',
              isDense: true,
            ),
            keyboardType:
                const TextInputType.numberWithOptions(decimal: true, signed: true),
            onChanged: _onLatTextChanged,
          ),
          const SizedBox(height: 8),
          TextField(
            controller: _lng,
            decoration: const InputDecoration(
              labelText: 'Longitude',
              isDense: true,
            ),
            keyboardType:
                const TextInputType.numberWithOptions(decimal: true, signed: true),
            onChanged: _onLngTextChanged,
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _desc,
            decoration:
                const InputDecoration(labelText: 'Area / Description'),
            maxLines: 3,
          ),
        ],
      ),
    );
  }
}

// ── Step 3: Details & Pricing ─────────────────────────────────────────────────

class _Step3 extends ConsumerStatefulWidget {
  const _Step3({required this.onNext});
  final VoidCallback onNext;

  @override
  ConsumerState<_Step3> createState() => _Step3State();
}

class _Step3State extends ConsumerState<_Step3> {
  late TextEditingController _ownerName, _ownerContact, _price, _plotArea, _builtUp;

  @override
  void initState() {
    super.initState();
    final s = ref.read(propertyFormProvider);
    _ownerName = TextEditingController(text: s.ownerName);
    _ownerContact = TextEditingController(text: s.ownerContact);
    _price = TextEditingController(text: s.price);
    _plotArea = TextEditingController(text: s.plotArea);
    _builtUp = TextEditingController(text: s.builtUpArea);
  }

  @override
  void dispose() {
    for (final c in [_ownerName, _ownerContact, _price, _plotArea, _builtUp]) {
      c.dispose();
    }
    super.dispose();
  }

  double? get _pricePerSqm {
    final p = double.tryParse(_price.text);
    final a = double.tryParse(_builtUp.text) ?? double.tryParse(_plotArea.text);
    if (p == null || a == null || a == 0) return null;
    return p / a;
  }

  @override
  Widget build(BuildContext context) {
    final notifier = ref.read(propertyFormProvider.notifier);

    // Sync controllers when autofill (from step 4) updates state externally.
    ref.listen<PropertyFormState>(propertyFormProvider, (prev, next) {
      if (_price.text != next.price) _price.text = next.price;
      if (_ownerContact.text != next.ownerContact) _ownerContact.text = next.ownerContact;
      if (_plotArea.text != next.plotArea) _plotArea.text = next.plotArea;
      if (_builtUp.text != next.builtUpArea) _builtUp.text = next.builtUpArea;
    });

    return _StepWrapper(
      onNext: () {
        notifier.update((s) => s.copyWith(
              ownerName: _ownerName.text,
              ownerContact: _ownerContact.text,
              price: _price.text,
              plotArea: _plotArea.text,
              builtUpArea: _builtUp.text,
            ));
        // Early-create the property so photos can be uploaded immediately on step 4.
        final s = ref.read(propertyFormProvider);
        if (s.createdPropertyId == null &&
            _ownerName.text.isNotEmpty &&
            _ownerContact.text.isNotEmpty &&
            (double.tryParse(_price.text) ?? 0) > 0) {
          notifier.createProperty();
        }
        widget.onNext();
      },
      child: StatefulBuilder(
        builder: (context, setInner) => Column(
          children: [
            TextField(
              controller: _ownerName,
              decoration: const InputDecoration(labelText: 'Owner Name *'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _ownerContact,
              decoration: const InputDecoration(labelText: 'Owner Contact *'),
              keyboardType: TextInputType.phone,
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _price,
              decoration: const InputDecoration(labelText: 'Price (₹) *'),
              keyboardType: TextInputType.number,
              onChanged: (_) => setInner(() {}),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _plotArea,
              decoration: const InputDecoration(labelText: 'Plot Area (sqm)'),
              keyboardType: TextInputType.number,
              onChanged: (_) => setInner(() {}),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _builtUp,
              decoration: const InputDecoration(labelText: 'Built-up Area (sqm)'),
              keyboardType: TextInputType.number,
              onChanged: (_) => setInner(() {}),
            ),
            if (_pricePerSqm != null) ...[
              const SizedBox(height: 8),
              Text(
                'Price/sqm: ${_inrFormat.format(_pricePerSqm!)}',
                style: TextStyle(
                  color: Theme.of(context).colorScheme.primary,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

// ── Step 4: Photos ────────────────────────────────────────────────────────────

class _Step4 extends ConsumerStatefulWidget {
  const _Step4({required this.onNext, this.editPropertyId});
  final VoidCallback onNext;
  final String? editPropertyId;

  @override
  ConsumerState<_Step4> createState() => _Step4State();
}

class _Step4State extends ConsumerState<_Step4> {
  // Tracks which photo indices are running OCR.
  final Set<int> _ocrLoading = {};

  Future<void> _runAutofill(BuildContext context, int photoIndex) async {
    final form = ref.read(propertyFormProvider);
    final propertyId = form.createdPropertyId ?? widget.editPropertyId;
    final photo = form.photos[photoIndex];

    if (propertyId == null || photo.photoId == null) return;

    setState(() => _ocrLoading.add(photoIndex));
    try {
      final dio = createDioClient();
      final res = await dio.post(
        '/api/properties/$propertyId/photos/${photo.photoId}/ocr',
      );
      final suggestions =
          (res.data['data']?['suggestions'] as Map<String, dynamic>? ?? {})
              .map((k, v) => MapEntry(k, v.toString()));

      if (!context.mounted) return;
      if (suggestions.isEmpty) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('No suggestions found in this photo.')),
        );
        return;
      }
      await _showAutofillDialog(context, suggestions);
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Autofill failed: $e'),
            backgroundColor: Theme.of(context).colorScheme.error,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _ocrLoading.remove(photoIndex));
    }
  }

  Future<void> _showAutofillDialog(
    BuildContext context,
    Map<String, String> suggestions,
  ) async {
    final price = suggestions['price'];
    final area = suggestions['area'];
    final phone = suggestions['phone'];

    final apply = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        title: const Text('Smart Autofill'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              'The following values were detected in the photo:',
              style: TextStyle(color: Colors.grey, fontSize: 13),
            ),
            const SizedBox(height: 12),
            if (price != null) _SuggestionRow(label: 'Price', value: price),
            if (area != null) _SuggestionRow(label: 'Area', value: area),
            if (phone != null) _SuggestionRow(label: 'Contact', value: phone),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Ignore'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Apply'),
          ),
        ],
      ),
    );

    if (apply == true) {
      ref.read(propertyFormProvider.notifier).update((s) => s.copyWith(
            price: price ?? s.price,
            plotArea: area ?? s.plotArea,
            ownerContact: phone ?? s.ownerContact,
          ));
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Fields updated from photo.')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final form = ref.watch(propertyFormProvider);
    final notifier = ref.read(propertyFormProvider.notifier);
    final effectivePropertyId = form.createdPropertyId ?? widget.editPropertyId;
    final canAutofill = effectivePropertyId != null;

    final hintText = canAutofill
        ? 'Add photos and tap ✨ Autofill to extract price, area & contact.'
        : 'Fill in Step 3 (owner name, contact, price) to enable Autofill.';

    return _StepWrapper(
      nextLabel: 'Continue',
      onNext: widget.onNext,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(hintText, style: const TextStyle(color: Colors.grey)),
          const SizedBox(height: 12),
          OutlinedButton.icon(
            icon: const Icon(Icons.add_photo_alternate_outlined),
            label: const Text('Add Photos'),
            onPressed: () async {
              final picker = ImagePicker();
              final images = await picker.pickMultiImage();
              final startIndex = ref.read(propertyFormProvider).photos.length;
              for (final img in images) {
                notifier.addPhoto(File(img.path));
              }
              // Auto-upload immediately if property already exists.
              final pid = ref.read(propertyFormProvider).createdPropertyId ?? widget.editPropertyId;
              if (pid != null) {
                final count = ref.read(propertyFormProvider).photos.length;
                for (var i = startIndex; i < count; i++) {
                  notifier.uploadPhoto(pid, i);
                }
              }
            },
          ),
          const SizedBox(height: 16),
          if (form.photos.isNotEmpty)
            GridView.builder(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 3,
                crossAxisSpacing: 6,
                mainAxisSpacing: 6,
                childAspectRatio: 0.75,
              ),
              itemCount: form.photos.length,
              itemBuilder: (_, i) {
                final photo = form.photos[i];
                final isOcrLoading = _ocrLoading.contains(i);
                final showAutofill = canAutofill &&
                    photo.status == PhotoUploadStatus.confirmed &&
                    photo.photoId != null;

                return Stack(
                  fit: StackFit.expand,
                  children: [
                    Column(
                      children: [
                        Expanded(
                          child: ClipRRect(
                            borderRadius: BorderRadius.circular(8),
                            child: photo.file != null
                                ? Image.file(photo.file!, fit: BoxFit.cover, width: double.infinity)
                                : Image.network(photo.cdnUrl!, fit: BoxFit.cover, width: double.infinity),
                          ),
                        ),
                        if (showAutofill)
                          SizedBox(
                            width: double.infinity,
                            height: 28,
                            child: isOcrLoading
                                ? const Center(
                                    child: SizedBox(
                                      width: 16,
                                      height: 16,
                                      child: CircularProgressIndicator(
                                          strokeWidth: 2),
                                    ),
                                  )
                                : TextButton.icon(
                                    style: TextButton.styleFrom(
                                      padding: EdgeInsets.zero,
                                      minimumSize: Size.zero,
                                      tapTargetSize:
                                          MaterialTapTargetSize.shrinkWrap,
                                    ),
                                    icon: const Icon(Icons.auto_fix_high,
                                        size: 14),
                                    label: const Text('Autofill',
                                        style: TextStyle(fontSize: 11)),
                                    onPressed: () =>
                                        _runAutofill(context, i),
                                  ),
                          ),
                      ],
                    ),
                    if (photo.status == PhotoUploadStatus.uploading)
                      Positioned(
                        bottom: showAutofill ? 28 : 0,
                        left: 0,
                        right: 0,
                        child: LinearProgressIndicator(value: photo.progress),
                      ),
                    if (photo.status == PhotoUploadStatus.confirmed)
                      const Positioned(
                        top: 4,
                        right: 4,
                        child: CircleAvatar(
                          radius: 10,
                          backgroundColor: Colors.green,
                          child:
                              Icon(Icons.check, size: 12, color: Colors.white),
                        ),
                      ),
                    if (photo.status == PhotoUploadStatus.failed)
                      const Positioned(
                        top: 4,
                        right: 4,
                        child: CircleAvatar(
                          radius: 10,
                          backgroundColor: Colors.red,
                          child: Icon(Icons.error,
                              size: 12, color: Colors.white),
                        ),
                      ),
                    Positioned(
                      top: 4,
                      left: 4,
                      child: GestureDetector(
                        onTap: () => notifier.removePhoto(i),
                        child: const CircleAvatar(
                          radius: 10,
                          backgroundColor: Colors.black54,
                          child: Icon(Icons.close,
                              size: 12, color: Colors.white),
                        ),
                      ),
                    ),
                  ],
                );
              },
            ),
        ],
      ),
    );
  }
}

class _SuggestionRow extends StatelessWidget {
  const _SuggestionRow({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        children: [
          SizedBox(
            width: 60,
            child: Text(label,
                style: const TextStyle(
                    fontWeight: FontWeight.w600, fontSize: 13)),
          ),
          Expanded(
            child: Text(value,
                style: const TextStyle(fontSize: 13),
                overflow: TextOverflow.ellipsis),
          ),
        ],
      ),
    );
  }
}

// ── Step 5: Tags ──────────────────────────────────────────────────────────────

class _Step5 extends ConsumerStatefulWidget {
  const _Step5({required this.onNext});
  final VoidCallback onNext;

  @override
  ConsumerState<_Step5> createState() => _Step5State();
}

class _Step5State extends ConsumerState<_Step5> {
  final _ctrl = TextEditingController();
  List<String> _suggestions = [];

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  Future<void> _fetchSuggestions(String q) async {
    if (q.isEmpty) {
      setState(() => _suggestions = []);
      return;
    }
    try {
      final dio = createDioClient();
      final res = await dio.get('/api/tags', queryParameters: {'q': q});
      setState(() =>
          _suggestions = (res.data['data'] as List).cast<String>());
    } catch (_) {
      setState(() => _suggestions = []);
    }
  }

  @override
  Widget build(BuildContext context) {
    final form = ref.watch(propertyFormProvider);
    final notifier = ref.read(propertyFormProvider.notifier);

    return _StepWrapper(
      onNext: widget.onNext,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          TextField(
            controller: _ctrl,
            decoration: const InputDecoration(
              labelText: 'Add Tag',
              suffixIcon: Icon(Icons.label_outline),
            ),
            onChanged: _fetchSuggestions,
            onSubmitted: (v) {
              if (v.trim().isNotEmpty) {
                notifier.addTag(v.trim());
                _ctrl.clear();
                setState(() => _suggestions = []);
              }
            },
          ),
          if (_suggestions.isNotEmpty)
            Container(
              margin: const EdgeInsets.only(top: 4),
              decoration: BoxDecoration(
                border: Border.all(color: Colors.grey.shade300),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Column(
                children: _suggestions.map((s) {
                  return ListTile(
                    dense: true,
                    title: Text(s),
                    onTap: () {
                      notifier.addTag(s);
                      _ctrl.clear();
                      setState(() => _suggestions = []);
                    },
                  );
                }).toList(),
              ),
            ),
          const SizedBox(height: 16),
          Wrap(
            spacing: 8,
            runSpacing: 4,
            children: form.tags.map((t) {
              return Chip(
                label: Text(t),
                onDeleted: () => notifier.removeTag(t),
              );
            }).toList(),
          ),
        ],
      ),
    );
  }
}

// ── Step 6: Review & Submit ───────────────────────────────────────────────────

class _Step6 extends ConsumerWidget {
  const _Step6({required this.onSuccess});
  final VoidCallback onSuccess;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final form = ref.watch(propertyFormProvider);
    final notifier = ref.read(propertyFormProvider.notifier);

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _reviewRow('Category', form.listingCategory),
          _reviewRow('Type', form.propertyType),
          _reviewRow('Direct Owner', form.isDirectOwner ? 'Yes' : 'No'),
          _reviewRow('Lat', form.locationLat),
          _reviewRow('Lng', form.locationLng),
          _reviewRow('Owner', form.ownerName),
          _reviewRow('Contact', form.ownerContact),
          _reviewRow('Price', form.price),
          if (form.plotArea.isNotEmpty) _reviewRow('Plot Area', '${form.plotArea} sqm'),
          if (form.builtUpArea.isNotEmpty) _reviewRow('Built-up', '${form.builtUpArea} sqm'),
          if (form.description.isNotEmpty) _reviewRow('Description', form.description),
          _reviewRow('Photos', '${form.photos.length} selected'),
          _reviewRow('Tags', form.tags.isEmpty ? 'None' : form.tags.join(', ')),
          if (form.submitError != null) ...[
            const SizedBox(height: 12),
            Text(
              'Error: ${form.submitError}',
              style: const TextStyle(color: Colors.red),
            ),
          ],
          const SizedBox(height: 24),
          ElevatedButton(
            onPressed: form.isSubmitting
                ? null
                : () async {
                    await notifier.submit();
                    final s = ref.read(propertyFormProvider);
                    if (s.createdPropertyId != null && s.submitError == null) {
                      onSuccess();
                    }
                  },
            child: form.isSubmitting
                ? const SizedBox(
                    height: 20,
                    width: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Text('Submit'),
          ),
        ],
      ),
    );
  }

  Widget _reviewRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 100,
            child: Text(label,
                style: const TextStyle(color: Colors.grey, fontSize: 13)),
          ),
          Expanded(child: Text(value)),
        ],
      ),
    );
  }
}

// ── Shared wrapper ────────────────────────────────────────────────────────────

class _StepWrapper extends StatelessWidget {
  const _StepWrapper({
    required this.child,
    required this.onNext,
    this.nextLabel = 'Next',
  });

  final Widget child;
  final VoidCallback onNext;
  final String nextLabel;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(child: SingleChildScrollView(child: child)),
          const SizedBox(height: 16),
          ElevatedButton(onPressed: onNext, child: Text(nextLabel)),
        ],
      ),
    );
  }
}

Widget _label(String text) => Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Text(text,
          style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 14)),
    );
