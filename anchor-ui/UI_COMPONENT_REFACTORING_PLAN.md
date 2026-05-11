# Anchor UI Component Refactoring Plan

## Executive Summary

Your React UI application has a solid foundation with modern technologies (React 19, TypeScript, Tailwind CSS, Radix UI) and good architectural patterns. However, there are significant opportunities to improve consistency and create reusable "Lego-like" components that will enhance maintainability and user experience.

## Key Findings

### ✅ **Current Strengths**
- **Type Safety**: Excellent use of TypeScript with generated API types
- **Modern Stack**: Well-structured use of TanStack Query, Router, and Radix UI
- **Code Generation**: Consistent API client generation from OpenAPI specs
- **Base Components**: Good foundation with `FormAlert`, `Page`, and `AnchorDataTable`

### ⚠️ **Critical Issues Identified**

## 1. Spacing & Layout Inconsistencies

### **Problem**: Mixed spacing patterns across components
- **Forms**: `gap-4` vs `gap-6` inconsistency (`RegisterForm.tsx:92` vs `LoginForm.tsx:64`)
- **Containers**: Mix of `p-[32px]` (Page.tsx:53) and `p-6` (standard)
- **Layout**: Inconsistent use of `gap-*` vs `space-*` utilities

### **Recommendation**: Standardize spacing system
```css
/* Proposed Standard */
Large containers: gap-6 (24px)
Form fields: gap-4 (16px)  
Small elements: gap-2 (8px)
Container padding: p-6 (replace p-[32px])
```

## 2. Dialog Pattern Fragmentation

### **Problem**: Three different dialog patterns used inconsistently
- **Pattern A**: `Dialog` component (ProductCreateDialog)
- **Pattern B**: `AlertDialog` component (ProductDeleteDialog)
- **Pattern C**: Multi-step dialogs with `VerticalStepper`

### **Recommendation**: Create standardized dialog components
```tsx
// Proposed Components
<ConfirmationDialog />     // For delete actions
<FormDialog />            // For create/edit forms  
<MultiStepDialog />       // For complex workflows
```

## 3. Significant Code Duplication

### **High-Impact Duplication Areas**:

#### **A. BasicInfoStep Components (90% identical)**
- `product/roles/steps/BasicInfoStep.tsx`
- `product/apikey/steps/BasicInfoStep.tsx`

**Lines 45-160**: Nearly identical structure, form handling, and layout patterns.

#### **B. Delete Dialog Pattern (85% identical)**
- `ProductDeleteDialog.tsx`
- `PlatformDeleteUserDialog.tsx`
- `DeleteProductRoleDialog.tsx`

**Lines 31-48**: Identical state management and mutation patterns.

#### **C. DataTable Boilerplate (70% identical)**
- `PlatformUserDatatable.tsx`
- `ProductDatatable.tsx`

**Lines 23-89**: Repeated pagination, sorting, and search logic.

## Proposed Reusable Component System

### 1. **Generic Dialog Components**

```tsx
// /components/common/dialogs/
<GenericDeleteDialog 
  entityType="product"
  entityName={product.name}
  displayFields={[
    { label: "Name", value: product.name },
    { label: "Created", value: formatDate(product.created_at) }
  ]}
  onDelete={() => deleteProduct(product.id)}
/>

<GenericCreateDialog
  title="Create Product"
  fields={[
    { name: "name", type: "text", required: true },
    { name: "description", type: "textarea" }
  ]}
  onSubmit={handleCreate}
/>
```

### 2. **Reusable Step Components**

```tsx
// /components/common/steps/
<GenericBasicInfoStep
  icon={KeyIcon}
  entityType="API Key"
  namePlaceholder="Enter API key name"
  descriptionPlaceholder="Describe the API key purpose"
  formData={formData}
  setFormData={setFormData}
  onNext={handleNext}
/>
```

### 3. **DataTable Abstraction**

```tsx
// /hooks/useDataTable.ts
const useDataTable = <T>({
  queryFn,
  defaultSort: { id: 'created_at', desc: true },
  searchMapping,
  filterMapping
}) => {
  // All common pagination, sorting, filtering logic
  return { data, loading, pagination, sorting, handlers }
}
```

### 4. **Standardized Form Patterns**

```tsx
// /components/common/forms/
<FormField 
  label="Email"
  name="email"
  validation="required|email"
  error={errors.email}
/>

<FormSection title="Basic Information">
  {/* Form fields with consistent spacing */}
</FormSection>
```

## Implementation Tasks

---

## Task 1: Fix Spacing Inconsistencies
**Priority**: High | **Effort**: 2-3 hours | **Impact**: High

### Prompt for Claude:
```
Fix spacing inconsistencies across the UI components. Please:

1. Standardize spacing patterns using this system:
   - Large containers: gap-6 (24px)
   - Form fields: gap-4 (16px)  
   - Small elements: gap-2 (8px)
   - Container padding: p-6 (replace any p-[32px])

2. Update these specific files:
   - src/components/common/Page.tsx: Change p-[32px] to p-6
   - src/components/auth/RegisterForm.tsx: Change gap-4 to gap-6 on line 92 to match LoginForm
   - src/components/common/datatable/AnchorDataTable.tsx: Replace space-* with gap-* classes consistently

3. Review and fix any other spacing inconsistencies you find while making these changes

4. Ensure all changes maintain existing functionality and don't break layouts
```

---

TODO: MAKE TRIGGER GENERIC TOO, EVERYONE DEFINE THE TRIGGER BUTTON 
## Task 2: Create GenericDeleteDialog Component
**Priority**: High | **Effort**: 4-6 hours | **Impact**: High

### Prompt for Claude:
```
Create a reusable DeleteDialog component to replace the duplicated delete dialog patterns. Please:

1. Create /src/components/common/dialogs/DeleteDialog.tsx with these features:
   - Generic TypeScript interface for different entity types
   - Configurable entity information display
   - Consistent loading states and error handling
   - Toast notifications on success/error
   - Support for custom warning messages

2. The component should accept this interface:
   ```tsx
   interface DeleteDialogProps<T> {
     trigger: React.ReactNode;
     entityType: string;
     entityName: string;
     entity: T;
     displayFields: Array<{
       label: string;
       value: string | React.ReactNode;
       condition?: boolean;
     }>;
     warningMessage?: string;
     onDelete: () => Promise<void>;
     onDeleted?: () => void;
   }
   ```

3. Replace these existing components with the new generic one:
   - src/components/product/ProductDeleteDialog.tsx
   - src/components/platform/PlatformDeleteUserDialog.tsx  
   - src/components/product/roles/DeleteProductRoleDialog.tsx
   - src/components/product/apikey/DeleteProductApiKeyDialog.tsx

4. Update all references to use the new component
5. Test that all delete functionality still works correctly
```

---

## Task 3: Create GenericBasicInfoStep Component
**Priority**: Medium | **Effort**: 6-8 hours | **Impact**: High

### Prompt for Claude:
```
Create a reusable GenericBasicInfoStep component to eliminate the duplication between role and API key basic info steps. Please:

1. Create /src/components/common/steps/GenericBasicInfoStep.tsx that abstracts the common patterns from:
   - src/components/product/roles/steps/BasicInfoStep.tsx
   - src/components/product/apikey/steps/BasicInfoStep.tsx

2. The component should support this configuration interface:
   ```tsx
   interface BasicInfoStepConfig {
     icon: LucideIcon;
     title: string;
     entityType: string;
     namePlaceholder: string;
     descriptionPlaceholder: string;
     helpContent?: {
       title: string;
       description: string;
     };
     additionalFields?: React.ReactNode;
     validation?: {
       nameValidation?: (value: string) => string | null;
       descriptionValidation?: (value: string) => string | null;
     };
   }
   ```

3. Replace the existing BasicInfoStep components with the new generic one
4. Ensure all existing functionality (validation, error handling, navigation) is preserved
5. Update the parent dialog components to use the new generic step
```

---

## Task 4: Create useDataTable Hook
**Priority**: Medium | **Effort**: 8-10 hours | **Impact**: Medium

### Prompt for Claude:
```
Create a reusable useDataTable hook to abstract the common datatable logic used across components. Please:

1. Create /src/hooks/useDataTable.ts that extracts common patterns from:
   - src/components/platform/PlatformUserDatatable.tsx
   - src/components/product/ProductDatatable.tsx

2. The hook should handle:
   - Pagination state management
   - Sorting state management  
   - Search and filtering with debouncing
   - Query parameter synchronization
   - Loading and error states
   - Data fetching with TanStack Query

3. Create a generic interface:
   ```tsx
   interface DataTableConfig<T, F> {
     queryFn: (params: DataTableParams) => UseQueryResult<T[]>;
     defaultSort: { id: string; desc: boolean };
     searchMapping: (search: string) => any;
     filterMapping: (filters: F) => any;
     pageSize?: number;
   }
   ```

4. Update the existing datatable components to use the new hook
5. Create utility functions for common column patterns (timestamps, actions, badges)
6. Ensure all existing functionality is preserved and performance is maintained
```

---

## Task 5: Create GenericCreateDialog Component  
**Priority**: Low | **Effort**: 6-8 hours | **Impact**: Medium

### Prompt for Claude:
```
Create a reusable GenericCreateDialog component to standardize create/edit form dialogs. Please:

1. Analyze the patterns in:
   - src/components/product/ProductCreateDialog.tsx
   - src/components/platform/PlatformAddInvitationDialog.tsx

2. Create /src/components/common/dialogs/GenericCreateDialog.tsx with:
   - Generic form handling with React Hook Form
   - Configurable field definitions
   - Consistent validation and error display
   - Loading states and success handling
   - Support for both create and edit modes

3. The component should support:
   ```tsx
   interface CreateDialogConfig<T> {
     title: string;
     description?: string;
     fields: FieldConfig[];
     initialData?: Partial<T>;
     onSubmit: (data: T) => Promise<void>;
     onSuccess?: () => void;
     submitLabel?: string;
     editMode?: boolean;
   }
   ```

4. Replace existing create dialogs with the new generic component
5. Ensure form validation and submission logic works correctly
```

---

## Task 6: Implement Form Field Components
**Priority**: Low | **Effort**: 4-6 hours | **Impact**: Medium

### Prompt for Claude:
```
Create standardized form field components to ensure consistency across all forms. Please:

1. Create /src/components/common/forms/ directory with these components:
   - FormField.tsx (wrapper for input + label + error)
   - FormSection.tsx (grouped form sections with consistent spacing)
   - FormActions.tsx (standardized button layouts)

2. Components should provide:
   - Consistent spacing and layout
   - Integrated error display using FormAlert
   - Accessibility features (proper labeling, ARIA attributes)
   - TypeScript support for validation

3. Update existing forms to use the new components:
   - LoginForm.tsx
   - RegisterForm.tsx  
   - Dialog forms
   - Step components

4. Ensure all forms maintain their current functionality while gaining consistency
```

---

## Expected Benefits

### **Quantitative Impact**
- **~40% reduction** in component code duplication
- **~60% faster** development of new similar components
- **Consistent spacing** across all UI elements
- **Unified user experience** for similar interactions

### **Qualitative Benefits**
- **Easier maintenance**: Single source of truth for common patterns
- **Better testing**: Centralized testing of shared behaviors
- **Developer experience**: Faster onboarding and development
- **Design consistency**: Automatic adherence to design patterns

## Implementation Order

1. **Task 1**: Fix spacing inconsistencies (immediate, low risk)
2. **Task 2**: GenericDeleteDialog (high impact, moderate risk)
3. **Task 3**: GenericBasicInfoStep (high impact, higher complexity)
4. **Task 4**: useDataTable hook (medium impact, complex)
5. **Task 5**: GenericCreateDialog (nice to have)
6. **Task 6**: Form field components (polish)

This plan provides a clear roadmap to transform your UI into a consistent, maintainable component system that will scale with your application's growth.
