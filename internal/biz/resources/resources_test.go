package resources

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"io"
	"sort"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type MockedReporterResourceRepository struct {
	mock.Mock
}
type MockedInventoryResourceRepository struct {
	mock.Mock
}

type MockAuthz struct {
	mock.Mock
}

type MockLookupResourcesStream struct {
	mock.Mock
	responses []*v1beta1.LookupResourcesResponse
	current   int
}

func (m *MockLookupResourcesStream) Recv() (*v1beta1.LookupResourcesResponse, error) {
	if m.current >= len(m.responses) {
		return nil, io.EOF
	}
	res := m.responses[m.current]
	m.current++
	return res, nil
}

func (m *MockLookupResourcesStream) Header() (metadata.MD, error) {
	args := m.Called()
	return args.Get(0).(metadata.MD), args.Error(1)
}

func (m *MockLookupResourcesStream) Trailer() metadata.MD {
	args := m.Called()
	return args.Get(0).(metadata.MD)
}

func (m *MockLookupResourcesStream) CloseSend() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockLookupResourcesStream) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

func (m *MockLookupResourcesStream) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockLookupResourcesStream) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

// Update the MockAuthz LookupResources method to match the exact signature
func (m *MockAuthz) LookupResources(ctx context.Context, request *v1beta1.LookupResourcesRequest) (grpc.ServerStreamingClient[v1beta1.LookupResourcesResponse], error) {
	args := m.Called(ctx, request)
	return args.Get(0).(grpc.ServerStreamingClient[v1beta1.LookupResourcesResponse]), args.Error(1)
}

func TestLookupResources_Success(t *testing.T) {
	ctx := context.TODO()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	authz := &MockAuthz{}

	req := &v1beta1.LookupResourcesRequest{
		ResourceType: &v1beta1.ObjectType{
			Namespace: "test-namespace",
			Name:      "test-resource",
		},
		Relation: "view",
		Subject: &v1beta1.SubjectReference{
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{
					Namespace: "user",
					Name:      "default",
				},
				Id: "user1",
			},
		},
	}

	mockResponses := []*v1beta1.LookupResourcesResponse{
		{
			Resource: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{
					Namespace: "test-namespace",
					Name:      "test-resource",
				},
				Id: "resource1",
			},
		},
		{
			Resource: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{
					Namespace: "test-namespace",
					Name:      "test-resource",
				},
				Id: "resource2",
			},
		},
	}

	// Set up mock stream
	mockStream := &MockLookupResourcesStream{
		responses: mockResponses,
	}
	mockStream.On("Recv").Return(mockResponses[0], nil).Once()
	mockStream.On("Recv").Return(mockResponses[1], nil).Once()
	mockStream.On("Recv").Return(nil, io.EOF).Once()
	mockStream.On("Context").Return(ctx)

	// Set up authz mock
	authz.On("LookupResources", ctx, req).Return(mockStream, nil)

	useCase := New(repo, inventoryRepo, authz, nil, "", log.DefaultLogger, false)
	stream, err := useCase.LookupResources(ctx, req)

	assert.Nil(t, err)
	assert.NotNil(t, stream)

	// Verify we can receive all responses
	res1, err := stream.Recv()
	assert.Nil(t, err)
	assert.Equal(t, "resource1", res1.Resource.Id)

	res2, err := stream.Recv()
	assert.Nil(t, err)
	assert.Equal(t, "resource2", res2.Resource.Id)

	// Verify EOF
	_, err = stream.Recv()
	assert.Equal(t, io.EOF, err)
}

func (r *MockedReporterResourceRepository) Create(ctx context.Context, resource *model.Resource) (*model.Resource, []*model.Resource, error) {
	args := r.Called(ctx, resource)
	return args.Get(0).(*model.Resource), args.Get(1).([]*model.Resource), args.Error(2)
}

func (r *MockedReporterResourceRepository) Update(ctx context.Context, resource *model.Resource, id uuid.UUID) (*model.Resource, []*model.Resource, error) {
	args := r.Called(ctx, resource, id)
	return args.Get(0).(*model.Resource), args.Get(1).([]*model.Resource), args.Error(2)
}

func (r *MockedReporterResourceRepository) Delete(ctx context.Context, id uuid.UUID) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterResourceId(ctx context.Context, id model.ReporterResourceId) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByInventoryIdAndReporter(ctx context.Context, inventoryId *uuid.UUID, reporterResourceId string, reporterType string) (*model.Resource, error) {
	args := r.Called(ctx, inventoryId, reporterResourceId, reporterType)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterResourceIdv1beta2(ctx context.Context, id model.ReporterResourceUniqueIndex) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByInventoryIdAndResourceType(ctx context.Context, inventoryId *uuid.UUID, resourceType string) (*model.Resource, error) {
	args := r.Called(ctx, inventoryId, resourceType)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterData(ctx context.Context, reporterId string, resourceId string) (*model.Resource, error) {
	args := r.Called(ctx, reporterId, resourceId)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) ListAll(ctx context.Context) ([]*model.Resource, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*model.Resource), args.Error(1)
}

func (r *MockedInventoryResourceRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.InventoryResource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.InventoryResource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByWorkspaceId(ctx context.Context, workspace_id string) ([]*model.Resource, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*model.Resource), args.Error(1)
}

func (m *MockAuthz) Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*kesselv1.GetReadyzResponse), args.Error(1)
}

func (m *MockAuthz) Check(ctx context.Context, namespace string, permission string, res *model.Resource, sub *v1beta1.SubjectReference) (v1beta1.CheckResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	args := m.Called(ctx, namespace, permission, res, sub)
	return args.Get(0).(v1beta1.CheckResponse_Allowed), args.Get(1).(*v1beta1.ConsistencyToken), args.Error(2)
}

func (m *MockAuthz) CheckForUpdate(ctx context.Context, namespace string, permission string, res *model.Resource, sub *v1beta1.SubjectReference) (v1beta1.CheckForUpdateResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	args := m.Called(ctx, namespace, permission, res, sub)
	return args.Get(0).(v1beta1.CheckForUpdateResponse_Allowed), args.Get(1).(*v1beta1.ConsistencyToken), args.Error(2)
}

func (m *MockAuthz) CreateTuples(ctx context.Context, req *v1beta1.CreateTuplesRequest) (*v1beta1.CreateTuplesResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*v1beta1.CreateTuplesResponse), args.Error(1)
}

func (m *MockAuthz) DeleteTuples(ctx context.Context, request *v1beta1.DeleteTuplesRequest) (*v1beta1.DeleteTuplesResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*v1beta1.DeleteTuplesResponse), args.Error(1)
}

func (m *MockAuthz) UnsetWorkspace(ctx context.Context, namespace, localResourceId, resourceType string) (*v1beta1.DeleteTuplesResponse, error) {
	args := m.Called(ctx, namespace, localResourceId, resourceType)
	return args.Get(0).(*v1beta1.DeleteTuplesResponse), args.Error(1)
}

func (m *MockAuthz) SetWorkspace(ctx context.Context, local_resource_id, workspace, namespace, name string, upsert bool) (*v1beta1.CreateTuplesResponse, error) {
	args := m.Called(ctx, local_resource_id, workspace, namespace, name, upsert)
	return args.Get(0).(*v1beta1.CreateTuplesResponse), args.Error(1)
}

func resource1() *model.Resource {
	return &model.Resource{
		ID:    uuid.UUID{},
		OrgId: "my-org",
		ResourceData: map[string]any{
			"foo": "bar",
		},
		ResourceType: "my-resource",
		WorkspaceId:  "my-workspace",
		Reporter: model.ResourceReporter{
			Reporter: model.Reporter{
				ReporterId:      "reporter_id",
				ReporterType:    "reporter_type",
				ReporterVersion: "1.0.2",
			},
			LocalResourceId: "foo-resource",
		},
		ConsoleHref: "/etc/console",
		ApiHref:     "/etc/api",
		Labels: model.Labels{
			{
				Key:   "label-1",
				Value: "value-1",
			},
			{
				Key:   "label-1",
				Value: "value-2",
			},
			{
				Key:   "label-xyz",
				Value: "value-xyz",
			},
		},
	}
}

func resource2() *model.Resource {
	return &model.Resource{
		ID:    uuid.UUID{},
		OrgId: "my-org2",
		ResourceData: map[string]any{
			"foo2": "bar2",
		},
		ResourceType: "my-resource2",
		WorkspaceId:  "my-workspace",
		Reporter: model.ResourceReporter{
			Reporter: model.Reporter{
				ReporterId:      "reporter_id",
				ReporterType:    "reporter_type",
				ReporterVersion: "1.0.2",
			},
			LocalResourceId: "foo-resource",
		},
		ConsoleHref: "/etc/console",
		ApiHref:     "/etc/api",
		Labels: model.Labels{
			{
				Key:   "label-2",
				Value: "value-2",
			},
			{
				Key:   "label-2",
				Value: "value-3",
			},
			{
				Key:   "label-xyz",
				Value: "value-xyz",
			},
		},
	}
}

func resource3() *model.Resource {
	return &model.Resource{
		ID:    uuid.UUID{},
		OrgId: "my-org3",
		ResourceData: map[string]any{
			"foo3": "bar3",
		},
		ResourceType: "my-resource33",
		WorkspaceId:  "my-workspace",
		Reporter: model.ResourceReporter{
			Reporter: model.Reporter{
				ReporterId:      "reporter_id",
				ReporterType:    "reporter_type",
				ReporterVersion: "1.0.2",
			},
			LocalResourceId: "foo-resource",
		},
		ConsoleHref: "/etc/console",
		ApiHref:     "/etc/api",
		Labels: model.Labels{
			{
				Key:   "label-3",
				Value: "value-3",
			},
			{
				Key:   "label-2",
				Value: "value-3",
			},
			{
				Key:   "label-xyz",
				Value: "value-xyz",
			},
		},
	}
}

func TestCreateReturnsDbError(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// DB Error
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestCreateReturnsDbErrorBackwardsCompatible(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Validates backwards compatibility, record was not found via new method
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// DB Error
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestCreateResourceAlreadyExists(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrResourceAlreadyExists)
	repo.AssertExpectations(t)
}

func TestCreateResourceAlreadyExistsBackwardsCompatible(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrRecordNotFound)
	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrResourceAlreadyExists)
	repo.AssertExpectations(t)
}

func TestCreateNewResource(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	returnedResource := model.Resource{
		ID: id,
	}

	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything).Return(&returnedResource, []*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	r, err := useCase.Create(ctx, resource)
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	repo.AssertExpectations(t)
}

func TestCreateNewResource_ConsistencyToken(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	m := &MockAuthz{}
	returnedResource := model.Resource{
		ID: id,
	}

	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything).Return(&returnedResource, []*model.Resource{}, nil)

	m.On("SetWorkspace", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&v1beta1.CreateTuplesResponse{ConsistencyToken: &v1beta1.ConsistencyToken{Token: "foo-bar-consistency-token"}}, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, []*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	r, err := useCase.Create(ctx, resource)

	assert.Nil(t, err)
	assert.Equal(t, "foo-bar-consistency-token", r.ConsistencyToken)
	repo.AssertExpectations(t)
}

func TestUpdateReturnsDbError(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// DB Error
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}
func TestUpdateReturnsDbErrorBackwardsCompatible(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// DB Error
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestUpdateNewResourceCreatesIt(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	returnedResource := model.Resource{
		ID: id,
	}

	// Resource doesn't exist
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything).Return(&returnedResource, []*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	repo.AssertExpectations(t)
}

func TestUpdateExistingResource(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	resource.ID = id

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	returnedResource := model.Resource{
		ID: id,
	}

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, []*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	assert.Equal(t, resource.ID, r.ID)
	repo.AssertExpectations(t)
}
func TestUpdateExistingResourceBackwardsCompatible(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	resource.ID = id

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	returnedResource := model.Resource{
		ID: id,
	}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, []*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	assert.Equal(t, resource.ID, r.ID)
	repo.AssertExpectations(t)
}

func TestDeleteReturnsDbError(t *testing.T) {
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}
func TestDeleteReturnsDbErrorBackwardsCompatible(t *testing.T) {
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// DB Error
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestDeleteNonexistentResource(t *testing.T) {
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrResourceNotFound)
	repo.AssertExpectations(t)
}

func TestDeleteResource(t *testing.T) {
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	ctx := context.TODO()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{
		ID: id,
	}, nil)
	repo.On("Delete", mock.Anything, (uuid.UUID)(id)).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)

	err = useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Nil(t, err)

	repo.AssertExpectations(t)
}

func TestDeleteResourceBackwardsCompatible(t *testing.T) {
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	ctx := context.TODO()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{
		ID: id,
	}, nil)
	repo.On("Delete", mock.Anything, (uuid.UUID)(id)).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)

	err = useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Nil(t, err)

	repo.AssertExpectations(t)
}

func TestCreateResource_PersistenceDisabled(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Mock as if persistence is not disabled, for assurance
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("Create", mock.Anything, mock.Anything).Return(nil, nil)

	disablePersistence := true
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, disablePersistence)

	// Create the resource
	r, err := useCase.Create(ctx, resource)
	assert.Nil(t, err)
	assert.Equal(t, resource, r)

	// Create the same resource again, should not return an error since persistence is disabled
	r, err = useCase.Create(ctx, resource)
	assert.Nil(t, err)
	assert.Equal(t, resource, r)

	// Assert that the repository methods were not called since persistence is disabled
	repo.AssertNotCalled(t, "FindByReporterData")
	repo.AssertNotCalled(t, "FindByReporterResourceId")
	repo.AssertNotCalled(t, "Create")
}

func TestUpdateResource_PersistenceDisabled(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Mock as if persistence is not disabled, for assurance
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	disablePersistence := true
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, disablePersistence)

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, resource, r)

	// Assert that the repository methods were not called since persistence is disabled
	repo.AssertNotCalled(t, "FindByReporterData")
	repo.AssertNotCalled(t, "FindByReporterResourceId")
	repo.AssertNotCalled(t, "Update")
	repo.AssertNotCalled(t, "Create")
}

func TestDeleteResource_PersistenceDisabled(t *testing.T) {
	ctx := context.TODO()

	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Mock as if persistence is not disabled, for assurance
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{
		ID: id,
	}, nil)
	repo.On("Delete", mock.Anything, (uint64)(33)).Return(&model.Resource{}, nil)

	disablePersistence := true
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, disablePersistence)

	err = useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Nil(t, err)

	// Assert that the repository methods were not called since persistence is disabled
	repo.AssertNotCalled(t, "FindByReporterResourceId")
	repo.AssertNotCalled(t, "Delete")
}

func TestCheck_MissingResource(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrRecordNotFound)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	allowed, err := useCase.Check(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.True(t, allowed)

	// check negative case
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, nil)
	allowed, err = useCase.Check(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheck_ResourceExistsError(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrUnsupportedDriver) // some random error

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	allowed, err := useCase.Check(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheck_ErrorWithKessel(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	m.On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, errors.New("failed during call to relations"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	allowed, err := useCase.Check(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheck_Allowed(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	allowed, err := useCase.Check(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.True(t, allowed)

	// check negative case
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, nil)
	allowed, err = useCase.Check(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheckForUpdate_ResourceExistsError(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrUnsupportedDriver) // some random error

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheckForUpdate_ErrorWithKessel(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, errors.New("failed during call to relations"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheckForUpdate_WorkspaceAllowed(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{ResourceType: "workspace"})

	assert.Nil(t, err)
	assert.True(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheckForUpdate_MissingResource_Allowed(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrRecordNotFound)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	// no consistency token being written.

	assert.Nil(t, err)
	assert.True(t, allowed)

	repo.AssertExpectations(t)

}

func TestCheckForUpdate_Allowed(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, []*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.True(t, allowed)

	// check negative case
	m.On("CheckForUpdate", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, nil)

	allowed, err = useCase.CheckForUpdate(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestListResourcesInWorkspace_Error(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{}, errors.New("failed querying"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.NotNil(t, err)
	assert.Nil(t, resource_chan)
	assert.Nil(t, err_chan)

	repo.AssertExpectations(t)
}

func TestListResourcesInWorkspace_NoResources(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	res := <-resource_chan
	assert.Nil(t, res) // expecting no resources

	assert.Empty(t, err_chan) // dont want any errors.

	repo.AssertExpectations(t)
}

func TestListResourcesInWorkspace_ResourcesAllowedTrue(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	resource := resource1()

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	res := <-resource_chan
	assert.Equal(t, resource, res) // expecting to get back resource1

	_, ok := <-resource_chan
	if ok {
		t.Error("resource_chan should have been closed")
	}

	assert.Empty(t, err_chan) // dont want any errors.

	// check negative case (not allowed)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, nil)
	resource_chan, err_chan, err = useCase.ListResourcesInWorkspace(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	res = <-resource_chan
	assert.Nil(t, res) // expecting no resource, as we are not allowed

	assert.Empty(t, err_chan) // dont want any errors.

	repo.AssertExpectations(t)
}

func TestListResourcesInWorkspace_MultipleResourcesAllowedTrue(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	resource := resource1()
	resource2 := resource2()
	resource3 := resource3()

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource, resource2, resource3}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	out := make([]*model.Resource, 3)
	out[0] = <-resource_chan
	out[1] = <-resource_chan
	out[2] = <-resource_chan

	in := []*model.Resource{resource, resource2, resource3}
	sort.Slice(in, func(i, j int) bool { return len(in[i].ResourceType) < len(in[j].ResourceType) })
	sort.Slice(out, func(i, j int) bool { return len(out[i].ResourceType) < len(out[j].ResourceType) })
	assert.Equal(t, in, out) // all 3 are there in any order

	_, ok := <-resource_chan
	if ok {
		t.Error("resource_chan should have been closed") // and there was no other resource
	}

	assert.Empty(t, err_chan) // dont want any errors.
}

// not authorized for the middle one and error on the third should just pass first
func TestListResourcesInWorkspace_MultipleResourcesOneFalseTwoTrueLastError(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	resource := resource1()
	resource2 := resource2()
	resource3 := resource3()
	theError := errors.New("failed calling relations")

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource, resource2, resource3}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", resource, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", resource2, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", resource3, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_UNSPECIFIED, &v1beta1.ConsistencyToken{}, theError)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	out_allowed := make([]*model.Resource, 1)
	out_allowed[0] = <-resource_chan

	in_allowed := []*model.Resource{resource2}
	sort.Slice(in_allowed, func(i, j int) bool { return len(in_allowed[i].ResourceType) < len(in_allowed[j].ResourceType) })
	sort.Slice(out_allowed, func(i, j int) bool { return len(out_allowed[i].ResourceType) < len(out_allowed[j].ResourceType) })
	assert.Equal(t, in_allowed, out_allowed) // all 3 are there in any order

	_, ok := <-resource_chan
	if ok {
		t.Error("resource_chan should have been closed") // and there was no other resource
	}

	backError := <-err_chan
	assert.Equal(t, theError, backError) // dont want any errors.
}

func TestListResourcesInWorkspace_ResourcesAllowedError(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &MockAuthz{}

	resource := resource1()

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource}, nil)
	m.On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, errors.New("failed calling relations"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	res := <-resource_chan
	assert.Nil(t, res) // expecting no resource, as we errored

	assert.NotEmpty(t, err_chan) // we want an errors.
}
