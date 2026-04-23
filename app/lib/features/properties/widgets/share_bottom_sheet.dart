// app/lib/features/properties/widgets/share_bottom_sheet.dart
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../core/network/dio_client.dart';

/// Shows a modal bottom sheet with a shareable property message.
/// Calls [POST /api/properties/:id/share] then lets the user copy or
/// open WhatsApp directly.
Future<void> showShareSheet(BuildContext context, String propertyId) {
  return showModalBottomSheet(
    context: context,
    isScrollControlled: true,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
    ),
    builder: (_) => _ShareBottomSheet(propertyId: propertyId),
  );
}

class _ShareBottomSheet extends StatefulWidget {
  const _ShareBottomSheet({required this.propertyId});
  final String propertyId;

  @override
  State<_ShareBottomSheet> createState() => _ShareBottomSheetState();
}

class _ShareBottomSheetState extends State<_ShareBottomSheet> {
  late final Dio _dio;
  late final TextEditingController _ctrl;
  bool _loading = true;
  String? _error;
  String _message = '';
  String _whatsapp = '';

  @override
  void initState() {
    super.initState();
    _dio = createDioClient();
    _ctrl = TextEditingController();
    _load();
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final response =
          await _dio.post('/api/properties/${widget.propertyId}/share');
      final data = response.data['data'] as Map<String, dynamic>;
      setState(() {
        _message = data['plain_text'] as String? ?? '';
        _whatsapp = data['whatsapp_message'] as String? ?? '';
        _ctrl.text = _message;
        _loading = false;
      });
    } on DioException catch (e) {
      setState(() {
        _error = e.response?.data?['message'] as String? ?? e.message;
        _loading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _openWhatsApp() async {
    final text = Uri.encodeComponent(_ctrl.text);
    final uri = Uri.parse('https://wa.me/?text=$text');
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    } else {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('WhatsApp is not installed')),
        );
      }
    }
  }

  void _copy() {
    Clipboard.setData(ClipboardData(text: _ctrl.text));
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('Copied to clipboard')),
    );
  }

  @override
  Widget build(BuildContext context) {
    final viewInsets = MediaQuery.of(context).viewInsets;

    return Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: viewInsets.bottom + 16,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Handle bar
          Center(
            child: Container(
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: Colors.grey[300],
                borderRadius: BorderRadius.circular(2),
              ),
            ),
          ),
          const SizedBox(height: 12),

          Text(
            'Share Property',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 16),

          if (_loading)
            const Center(child: CircularProgressIndicator())
          else if (_error != null)
            Text(
              _error!,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            )
          else ...[
            // Editable message preview
            TextField(
              controller: _ctrl,
              maxLines: 8,
              decoration: const InputDecoration(
                border: OutlineInputBorder(),
                labelText: 'Message preview (editable)',
              ),
            ),
            const SizedBox(height: 12),

            // Swap between plain and WhatsApp variants
            Row(
              children: [
                TextButton.icon(
                  icon: const Icon(Icons.text_fields),
                  label: const Text('Plain text'),
                  onPressed: () => setState(() => _ctrl.text = _message),
                ),
                TextButton.icon(
                  icon: const Icon(Icons.chat),
                  label: const Text('WhatsApp format'),
                  onPressed: () => setState(() => _ctrl.text = _whatsapp),
                ),
              ],
            ),
            const SizedBox(height: 8),

            // Action buttons
            Row(
              children: [
                Expanded(
                  child: OutlinedButton.icon(
                    icon: const Icon(Icons.copy),
                    label: const Text('Copy'),
                    onPressed: _copy,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: FilledButton.icon(
                    icon: const Icon(Icons.send),
                    label: const Text('WhatsApp'),
                    style: FilledButton.styleFrom(
                      backgroundColor: const Color(0xFF25D366),
                    ),
                    onPressed: _openWhatsApp,
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}
