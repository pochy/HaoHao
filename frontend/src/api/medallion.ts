import {
  getMedallionResourceCatalog,
  listMedallionAssets,
} from './generated/sdk.gen'
import type {
  MedallionAssetBody,
  MedallionCatalogBody,
} from './generated/types.gen'

export type MedallionResourceKind = 'drive_file' | 'dataset' | 'work_table' | 'ocr_run' | 'product_extraction' | 'gold_table'

export async function fetchMedallionResourceCatalog(resourceKind: MedallionResourceKind, resourcePublicId: string): Promise<MedallionCatalogBody> {
  return getMedallionResourceCatalog({
    path: { resourceKind, resourcePublicId },
  }) as unknown as Promise<MedallionCatalogBody>
}

export async function fetchMedallionAssets(layer = '', resourceKind = '', limit = 100, q = ''): Promise<MedallionAssetBody[]> {
  const data = await listMedallionAssets({
    query: {
      ...(q ? { q } : {}),
      ...(layer ? { layer: layer as 'bronze' | 'silver' | 'gold' } : {}),
      ...(resourceKind ? { resourceKind: resourceKind as MedallionResourceKind } : {}),
      limit,
    },
  }) as unknown as { items?: MedallionAssetBody[] | null }
  return data.items ?? []
}
